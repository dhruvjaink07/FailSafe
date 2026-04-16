package preview

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"
)

// MJPEGPreviewHandler streams repeated screenshots (PNG) as multipart/x-mixed-replace.
// It uses `adb exec-out screencap -p` to capture the emulator/device screen on the host.
// This is a pragmatic prototype for live preview that works in browsers via an <img> tag.
//
// Notes and limitations (see docs/preview.md):
//   - Lower performance than WebRTC but much easier to implement reliably across envs.
//   - Requires `adb` to be available on the host and the emulator/device connected to adb.
//   - To integrate scrcpy or a WebRTC pipeline later, replace the capture loop with an encoder
//     that feeds frames into a Pion PeerConnection.
func MJPEGPreviewHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Simple parameter: optional `device` query to select adb target (emulator serial)
		device := r.URL.Query().Get("device")

		boundary := "--failsafe-mjpeg-boundary"
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
		w.WriteHeader(http.StatusOK)

		// flushable writer
		flusher, _ := w.(http.Flusher)

		// capture loop: repeatedly run `adb exec-out screencap -p` and stream PNG frames
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Build adb command
			var cmd *exec.Cmd
			if device != "" {
				cmd = exec.Command("adb", "-s", device, "exec-out", "screencap", "-p")
			} else {
				cmd = exec.Command("adb", "exec-out", "screencap", "-p")
			}

			// run command and capture output
			out, err := cmd.Output()
			if err != nil {
				// write an error frame as plain text and exit the stream
				log.Printf("mjpeg: capture error: %v", err)
				_, _ = fmt.Fprintf(w, "--%s\r\nContent-Type: text/plain\r\n\r\nERROR: %v\r\n", boundary, err)
				flusher.Flush()
				return
			}

			// write multipart frame
			_, _ = fmt.Fprintf(w, "--%s\r\nContent-Type: image/png\r\nContent-Length: %d\r\n\r\n", boundary, len(out))
			if _, err := w.Write(out); err != nil {
				return
			}
			_, _ = w.Write([]byte("\r\n"))
			if flusher != nil {
				flusher.Flush()
			}

			// throttle capture rate to ~8-10 fps to avoid overloading CPU
			time.Sleep(120 * time.Millisecond)
		}
	}
}
