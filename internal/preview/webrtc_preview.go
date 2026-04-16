package preview

import (
	"bufio"
	"context"
	cryptoRand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus"
)

// This package implements a simple WebRTC publisher prototype using Pion.
// It spawns an ffmpeg process that reads screenshots from adb and encodes
// an H264 stream, which we parse and forward as RTP to a Pion video track.
//
// Notes:
// - This is a prototype: error handling and resource cleanup are pragmatic.

type sdpRequest struct {
	SDP    string `json:"sdp"`
	Device string `json:"device"` // optional adb serial
}

type sdpResponse struct {
	SDP       string `json:"sdp"`
	SessionID string `json:"session_id"`
}

// session represents a running preview session
type session struct {
	pc        *webrtc.PeerConnection
	cancel    context.CancelFunc
	ssrc      uint32
	packetCnt uint32
	octetCnt  uint32
	rrMu      sync.Mutex
	lastRR    *rtcp.ReceiverReport
	lastRRAt  time.Time
}

var (
	sessions   = map[string]*session{}
	sessionsMu sync.Mutex
)

// Prometheus metrics (labeled by ssrc)
var (
	promPacketCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "failsafe_preview_packet_count",
		Help: "Total RTP packets sent for preview session",
	}, []string{"ssrc"})
	promOctetCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "failsafe_preview_octet_count",
		Help: "Total payload octets sent for preview session",
	}, []string{"ssrc"})
	promLastRRFraction = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "failsafe_preview_last_rr_fraction",
		Help: "Last Receiver Report fraction lost for preview session",
	}, []string{"ssrc"})
	promLastRRLastSeq = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "failsafe_preview_last_rr_last_seq",
		Help: "Last Receiver Report last sequence number",
	}, []string{"ssrc"})
)

func init() {
	prometheus.MustRegister(promPacketCount, promOctetCount, promLastRRFraction, promLastRRLastSeq)
}

// StartPreviewHandler accepts an SDP offer and returns an SDP answer.
// It also starts an ffmpeg pipeline that reads from adb and writes H264 NALs
// to the PeerConnection video track.
func StartPreviewHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req sdpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, "invalid json", 400)
			return
		}

		// Create a new PeerConnection
		peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			httpError(w, err.Error(), 500)
			return
		}

		// RTCP receiver handling will be started after session creation (so it can store per-session metrics)

		// Create a video track. We will write RTP packets into this track.
		videoTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
		if err != nil {
			httpError(w, err.Error(), 500)
			return
		}

		if _, err = peerConnection.AddTrack(videoTrack); err != nil {
			httpError(w, err.Error(), 500)
			return
		}

		// Set remote description (offer)
		offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: req.SDP}
		if err := peerConnection.SetRemoteDescription(offer); err != nil {
			httpError(w, err.Error(), 500)
			return
		}

		// Create answer
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			httpError(w, err.Error(), 500)
			return
		}
		if err := peerConnection.SetLocalDescription(answer); err != nil {
			httpError(w, err.Error(), 500)
			return
		}

		// generate per-session SSRC
		var ssrc uint32
		{
			var b [4]byte
			if _, err := cryptoRand.Read(b[:]); err != nil {
				ssrc = uint32(time.Now().UnixNano() & 0xffffffff)
			} else {
				ssrc = binary.BigEndian.Uint32(b[:])
			}
		}

		// Start ffmpeg pipeline and feed H264 NALs into the track
		ctx, cancel := context.WithCancel(context.Background())

		// create session object and store early so RTCP goroutine can use it
		sess := &session{pc: peerConnection, cancel: cancel, ssrc: ssrc}

		// RTCP sender report goroutine

		// If PeerConnection supports ReadRTCP(), spawn a goroutine to read incoming RTCP
		type rtcpReader interface {
			ReadRTCP() ([]rtcp.Packet, error)
		}
		if rr, ok := interface{}(peerConnection).(rtcpReader); ok {
			go func(s *session) {
				for {
					pkts, err := rr.ReadRTCP()
					if err != nil {
						log.Printf("preview: ReadRTCP error: %v", err)
						return
					}
					for _, p := range pkts {
						switch rpt := p.(type) {
						case *rtcp.ReceiverReport:
							// store the latest ReceiverReport into the session
							if len(rpt.Reports) > 0 {
								s.rrMu.Lock()
								// copy the report header and first reception report
								first := rpt.Reports[0]
								copyRpt := &rtcp.ReceiverReport{SSRC: rpt.SSRC, Reports: []rtcp.ReceptionReport{first}}
								s.lastRR = copyRpt
								s.lastRRAt = time.Now()
								s.rrMu.Unlock()
								log.Printf("preview: stored RR for session %x fraction=%d lost=%d lastSeq=%d", rpt.SSRC, first.FractionLost, first.TotalLost, first.LastSequenceNumber)
								// update prometheus gauges
								promLastRRFraction.WithLabelValues(fmt.Sprintf("%x", rpt.SSRC)).Set(float64(first.FractionLost))
								promLastRRLastSeq.WithLabelValues(fmt.Sprintf("%x", rpt.SSRC)).Set(float64(first.LastSequenceNumber))
							}
						default:
							// ignore other RTCP packets here
						}
					}
				}
			}(sess)
		} else {
			log.Printf("preview: PeerConnection does not expose ReadRTCP(), skipping RTCP receiver handling")
		}

		// RTCP sender report goroutine
		go func(s *session) {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Build Sender Report
					now := time.Now()
					sec := uint64(now.Unix()) + 2208988800
					frac := uint64((uint64(now.Nanosecond()) * (1 << 32)) / 1000000000)
					ntp := (sec << 32) | frac
					sr := &rtcp.SenderReport{
						SSRC:        s.ssrc,
						NTPTime:     ntp,
						RTPTime:     uint32((now.UnixNano() / 1e6) * 90),
						PacketCount: atomic.LoadUint32(&s.packetCnt),
						OctetCount:  atomic.LoadUint32(&s.octetCnt),
					}
					_ = s.pc.WriteRTCP([]rtcp.Packet{sr})
				}
			}
		}(sess)

		go func() {
			// Build the ffmpeg command that reads PNG frames from adb and outputs raw H264
			// Example pipeline:
			// adb exec-out screencap -p | ffmpeg -f image2pipe -vcodec png -i - -c:v libx264 -preset ultrafast -tune zerolatency -f h264 -
			adbArgs := []string{"exec-out", "screencap", "-p"}
			if strings.TrimSpace(req.Device) != "" {
				adbArgs = []string{"-s", req.Device, "exec-out", "screencap", "-p"}
			}

			adbCmd := exec.CommandContext(ctx, "adb", adbArgs...)
			ffmpegCmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error",
				"-f", "image2pipe", "-vcodec", "png", "-i", "-",
				"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
				"-pix_fmt", "yuv420p", "-an", "-f", "h264", "-")

			// pipe adb -> ffmpeg stdin
			adbStdout, err := adbCmd.StdoutPipe()
			if err != nil {
				log.Printf("preview: adb stdout pipe error: %v", err)
				cancel()
				return
			}
			ffmpegCmd.Stdin = adbStdout

			ffOut, err := ffmpegCmd.StdoutPipe()
			if err != nil {
				log.Printf("preview: ffmpeg stdout pipe error: %v", err)
				cancel()
				return
			}
			// capture ffmpeg stderr for debugging
			ffErr, err := ffmpegCmd.StderrPipe()
			if err != nil {
				log.Printf("preview: ffmpeg stderr pipe error: %v", err)
				cancel()
				return
			}

			if err := adbCmd.Start(); err != nil {
				log.Printf("preview: adb start error: %v", err)
				cancel()
				return
			}
			if err := ffmpegCmd.Start(); err != nil {
				log.Printf("preview: ffmpeg start error: %v", err)
				_ = adbCmd.Process.Kill()
				cancel()
				return
			}

			// log ffmpeg stderr lines
			go func() {
				scanner := bufio.NewScanner(ffErr)
				for scanner.Scan() {
					log.Printf("preview ffmpeg: %s", scanner.Text())
				}
				if err := scanner.Err(); err != nil {
					log.Printf("preview: ffmpeg stderr read error: %v", err)
				}
			}()

			// Read raw H264 stream from ffmpeg and split by NAL start codes
			reader := bufio.NewReader(ffOut)

			// Simple NAL reader: look for 0x000001 or 0x00000001 sequences
			var nalBuf []byte
			for {
				// Read chunk
				chunk := make([]byte, 4096)
				n, err := reader.Read(chunk)
				if n > 0 {
					nalBuf = append(nalBuf, chunk[:n]...)
					// extract NAL units
					for {
						idx := findStartCode(nalBuf)
						if idx == -1 {
							break
						}
						// find next start code after idx+3
						next := findStartCode(nalBuf[idx+3:])
						if next == -1 {
							// wait for more data
							break
						}
						// next is offset from idx+3
						nextIdx := idx + 3 + next
						nal := nalBuf[idx:nextIdx]
						// write nal (packetization happens in writeH264NAL)
						if err := writeH264NAL(videoTrack, nal, sess.ssrc, &sess.packetCnt, &sess.octetCnt); err != nil {
							log.Printf("preview: writeH264NAL error: %v", err)
						}
						// advance buffer
						nalBuf = nalBuf[nextIdx:]
					}
				}
				if err != nil {
					if err == io.EOF {
						break
					}
					log.Printf("preview: ffmpeg read error: %v", err)
					break
				}
			}

			// cleanup
			_ = ffmpegCmd.Wait()
			_ = adbCmd.Wait()
			cancel()
		}()

		// store session keyed by local description's sdp (simple unique key)
		sid := fmt.Sprintf("sess-%d", time.Now().UnixNano())
		sessionsMu.Lock()
		sessions[sid] = sess
		sessionsMu.Unlock()

		// return answer SDP and session id (client will set remote)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdpResponse{SDP: peerConnection.LocalDescription().SDP, SessionID: sid})
	}
}

// SessionMetricsHandler returns per-session metrics (packet/octet counts and last RR)
func SessionMetricsHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sid := q.Get("session_id")
		if sid == "" {
			httpError(w, "session_id required", 400)
			return
		}
		sessionsMu.Lock()
		s, ok := sessions[sid]
		sessionsMu.Unlock()
		if !ok {
			httpError(w, "session not found", 404)
			return
		}
		s.rrMu.Lock()
		var rr *rtcp.ReceiverReport
		if s.lastRR != nil {
			copyRpt := *s.lastRR
			rr = &copyRpt
		}
		rrAt := s.lastRRAt
		s.rrMu.Unlock()

		resp := map[string]any{
			"session_id":   sid,
			"ssrc":         fmt.Sprintf("%x", s.ssrc),
			"packet_count": atomic.LoadUint32(&s.packetCnt),
			"octet_count":  atomic.LoadUint32(&s.octetCnt),
			"last_rr":      rr,
			"last_rr_at":   rrAt.Format(time.RFC3339Nano),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// ListSessionsHandler returns a JSON array of active session summaries
func ListSessionsHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionsMu.Lock()
		defer sessionsMu.Unlock()
		out := make([]map[string]any, 0, len(sessions))
		for sid, s := range sessions {
			s.rrMu.Lock()
			var rr *rtcp.ReceiverReport
			if s.lastRR != nil {
				copyRpt := *s.lastRR
				rr = &copyRpt
			}
			rrAt := s.lastRRAt
			s.rrMu.Unlock()

			out = append(out, map[string]any{
				"session_id":   sid,
				"ssrc":         fmt.Sprintf("%x", s.ssrc),
				"packet_count": atomic.LoadUint32(&s.packetCnt),
				"octet_count":  atomic.LoadUint32(&s.octetCnt),
				"last_rr":      rr,
				"last_rr_at":   rrAt.Format(time.RFC3339Nano),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

// StopPreviewHandler stops a running session given a session id
func StopPreviewHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sid := q.Get("session_id")
		if sid == "" {
			httpError(w, "session_id required", 400)
			return
		}
		sessionsMu.Lock()
		s, ok := sessions[sid]
		if ok {
			s.cancel()
			_ = s.pc.Close()
			delete(sessions, sid)
		}
		sessionsMu.Unlock()
		w.WriteHeader(204)
	}
}

func httpError(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	_, _ = w.Write([]byte(msg))
}

// findStartCode returns the index of the first start code (0x000001 or 0x00000001) or -1
func findStartCode(b []byte) int {
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0x00 && b[i+1] == 0x00 && b[i+2] == 0x01 {
			return i
		}
		if i+4 < len(b) && b[i] == 0x00 && b[i+1] == 0x00 && b[i+2] == 0x00 && b[i+3] == 0x01 {
			return i
		}
	}
	return -1
}

// writeH264NAL wraps a single NAL into a minimal RTP packet and writes to the track.
// This is a naive approach and not suitable for production; it's a prototype.
func writeH264NAL(track *webrtc.TrackLocalStaticRTP, nal []byte, ssrc uint32, packetCnt *uint32, octetCnt *uint32) error {
	// strip start code if present
	payload := stripStartCode(nal)

	// RTP parameters
	const mtu = 1200
	// RTP header ~12 bytes, FU-A header 2 bytes. Keep payload conservative.
	maxPayload := mtu - 12 - 2

	// timestamp in 90kHz clock
	ts := uint32((time.Now().UnixNano() / 1e6) * 90)

	// if payload fits in one RTP packet, send it as-is
	if len(payload) <= maxPayload {
		seq := uint16(atomic.AddUint32(&globalSeq, 1) & 0xffff)
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				SSRC:           ssrc,
				SequenceNumber: seq,
				Timestamp:      ts,
				Marker:         true,
			},
			Payload: payload,
		}
		if err := track.WriteRTP(pkt); err != nil {
			return err
		}
		if packetCnt != nil {
			atomic.AddUint32(packetCnt, 1)
			promPacketCount.WithLabelValues(fmt.Sprintf("%x", ssrc)).Set(float64(atomic.LoadUint32(packetCnt)))
		}
		if octetCnt != nil {
			atomic.AddUint32(octetCnt, uint32(len(payload)))
			promOctetCount.WithLabelValues(fmt.Sprintf("%x", ssrc)).Set(float64(atomic.LoadUint32(octetCnt)))
		}
		return nil
	}

	// Fragment into FU-A packets per RFC 6184
	nalHeader := payload[0]
	nalData := payload[1:]
	fuIndicator := (nalHeader & 0xE0) | 28 // F and NRI from nalHeader, type=28 (FU-A)

	// iterate fragments
	offset := 0
	first := true
	for offset < len(nalData) {
		remain := len(nalData) - offset
		chunkSize := maxPayload
		if remain < chunkSize {
			chunkSize = remain
		}

		fuHeader := byte(nalHeader & 0x1F)
		var s, e byte
		if first {
			s = 0x80 // Start bit
			first = false
		}
		if offset+chunkSize >= len(nalData) {
			e = 0x40 // End bit
		}

		header := []byte{fuIndicator, s | e | fuHeader}
		fragment := append(header, nalData[offset:offset+chunkSize]...)

		seq := uint16(atomic.AddUint32(&globalSeq, 1) & 0xffff)
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				SSRC:           ssrc,
				SequenceNumber: seq,
				Timestamp:      ts,
				Marker:         (e != 0),
			},
			Payload: fragment,
		}
		if err := track.WriteRTP(pkt); err != nil {
			return err
		}
		if packetCnt != nil {
			atomic.AddUint32(packetCnt, 1)
			promPacketCount.WithLabelValues(fmt.Sprintf("%x", ssrc)).Set(float64(atomic.LoadUint32(packetCnt)))
		}
		if octetCnt != nil {
			atomic.AddUint32(octetCnt, uint32(len(fragment)))
			promOctetCount.WithLabelValues(fmt.Sprintf("%x", ssrc)).Set(float64(atomic.LoadUint32(octetCnt)))
		}

		offset += chunkSize
	}

	return nil
}

var globalSeq uint32

// stripStartCode removes a leading H264 start code (0x000001 or 0x00000001) if present
func stripStartCode(b []byte) []byte {
	if len(b) >= 4 && b[0] == 0x00 && b[1] == 0x00 && b[2] == 0x00 && b[3] == 0x01 {
		return b[4:]
	}
	if len(b) >= 3 && b[0] == 0x00 && b[1] == 0x00 && b[2] == 0x01 {
		return b[3:]
	}
	return b
}
