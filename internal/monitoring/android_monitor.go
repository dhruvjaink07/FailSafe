package monitoring

import (
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
	"github.com/dhruvjaink07/failsafe/internal/models"
)

// AndroidMonitor observes app/system behavior via ADB
type AndroidMonitor struct {
	adb              *adb.Client
	pkg              string
	callback         func(EventType, models.MetricSample)
	stop             chan struct{}
	currentIntensity int
	previousState    string
	healthyStreak    int
	unhealthyActive  bool
	lastLogAt        time.Time
}

// SetIntensity implements [MonitorInterface].
func (m *AndroidMonitor) SetIntensity(i int) {
	m.currentIntensity = i
}

// Constructor
func NewAndroidMonitorWithCallback(
	cb func(EventType, models.MetricSample),
	adbClient *adb.Client,
	pkg string,
) *AndroidMonitor {
	return &AndroidMonitor{
		adb:      adbClient,
		pkg:      pkg,
		callback: cb,
		stop:     make(chan struct{}),
	}
}

//
// ---------------- CRASH DETECTION ----------------
//

// Checks if app crashed using logcat
func (m *AndroidMonitor) HasCrash() bool {
	events := m.readStructuredEvents()
	for _, ev := range events {
		if ev.Type == "crash" {
			return true
		}
	}
	return false
}

func (m *AndroidMonitor) readStructuredEvents() []LogEvent {
	now := time.Now()
	if m.lastLogAt.IsZero() {
		m.lastLogAt = now.Add(-10 * time.Second)
	}

	timeAnchor := m.lastLogAt.Format("2006-01-02 15:04:05.000")
	raw := ""

	if pid, ok := m.getAppPID(); ok {
		out, err := m.adb.Shell("logcat -d --pid=" + pid + " -T \"" + timeAnchor + "\"")
		if err == nil {
			raw = out
		}
	}

	if strings.TrimSpace(raw) == "" {
		out, err := m.adb.Shell("logcat -d -T \"" + timeAnchor + "\" | grep " + m.pkg)
		if err == nil {
			raw = out
		}
	}

	m.lastLogAt = now
	return parseLogcatEvents(raw, now)
}

func (m *AndroidMonitor) getAppPID() (string, bool) {
	out, err := m.adb.Shell("pidof " + m.pkg)
	if err != nil {
		return "", false
	}

	pids := strings.Fields(strings.TrimSpace(out))
	if len(pids) == 0 {
		return "", false
	}

	return pids[0], true
}

//
// ---------------- ANR DETECTION ----------------
//

// Checks if app has ANR (App Not Responding)
func (m *AndroidMonitor) HasANR() bool {

	out, err := m.adb.Shell("cat /data/anr/traces.txt")
	if err != nil {
		return false
	}

	// If traces exist → ANR happened
	return len(strings.TrimSpace(out)) > 0
}

//
// ---------------- MEMORY USAGE ----------------
//

// Returns raw memory info (you will parse later)
func (m *AndroidMonitor) Memory() string {

	out, err := m.adb.Dumpsys("meminfo " + m.pkg)
	if err != nil {
		return ""
	}

	return out
}

//
// ---------------- APP STATE ----------------
//

// Check if app is in foreground
func (m *AndroidMonitor) IsAppRunning() bool {

	out, err := m.adb.Dumpsys("activity activities")
	if err != nil {
		return false
	}

	return strings.Contains(out, m.pkg)
}

//
// ---------------- CPU (basic) ----------------
//

// Simple CPU snapshot
func (m *AndroidMonitor) CPU() string {

	out, err := m.adb.Shell("top -n 1 | grep " + m.pkg)
	if err != nil {
		return ""
	}

	return out
}

func (m *AndroidMonitor) Start(id string, endpoints []string) {

	go func() {
		_, _ = m.adb.Shell("logcat -c")
		m.lastLogAt = time.Now()

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {

			case <-ticker.C:

				events := m.readStructuredEvents()
				crash := false
				anr := false
				crashReason := ""
				crashThread := ""

				for _, ev := range events {
					if ev.Type == "crash" {
						crash = true
						if crashReason == "" {
							crashReason = ev.Message
							crashThread = ev.Thread
						}
					}
					if ev.Type == "anr" {
						anr = true
					}
				}

				if !anr {
					anr = m.HasANR()
				}
				running := m.IsAppRunning()

				state := "running"
				if crash {
					state = "crash"
				} else if anr {
					state = "anr"
				} else if !running {
					state = "not_running"
				}

				sample := models.MetricSample{
					Endpoint:    m.pkg,
					Timestamp:   time.Now(),
					Crash:       crash,
					CrashReason: crashReason,
					CrashThread: crashThread,
					ANR:         anr,
					AppState:    state,
					Intensity:   m.currentIntensity,
				}

				event := EventType("sample")

				if m.previousState != "" && m.previousState != state {
					if state == "running" {
						m.healthyStreak = 1
					} else {
						m.healthyStreak = 0
						m.unhealthyActive = true
						if state == "not_running" {
							event = EventDegraded
						} else {
							event = EventDown
						}
					}
				} else if state == "running" {
					if m.unhealthyActive {
						m.healthyStreak++
						if m.healthyStreak >= 2 {
							event = EventRecovered
							m.unhealthyActive = false
							m.healthyStreak = 0
						}
					}
				} else {
					m.healthyStreak = 0
					m.unhealthyActive = true
				}

				m.previousState = state

				m.callback(event, sample)

			case <-m.stop:
				return
			}
		}
	}()
}

func (m *AndroidMonitor) Stop() {
	close(m.stop)
}
