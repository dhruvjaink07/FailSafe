package monitoring

import (
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
	"github.com/dhruvjaink07/failsafe/internal/models"
)

// AndroidMonitor observes app/system behavior via ADB
type AndroidMonitor struct {
	adb      *adb.Client
	pkg      string
	callback func(EventType, models.MetricSample)
	stop     chan struct{}
}

// SetIntensity implements [MonitorInterface].
func (m *AndroidMonitor) SetIntensity(i int) {
	panic("unimplemented")
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

	out, err := m.adb.Logcat()
	if err != nil {
		return false
	}

	// Basic signal for crash
	return strings.Contains(out, "FATAL EXCEPTION")
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

		ticker := time.NewTicker(2 * time.Second)

		for {
			select {

			case <-ticker.C:

				crash := m.HasCrash()
				anr := m.HasANR()
				running := m.IsAppRunning()

				state := "running"
				if !running {
					state = "not_running"
				}

				sample := models.MetricSample{
					Endpoint:  m.pkg,
					Timestamp: time.Now(),
					Crash:     crash,
					ANR:       anr,
					AppState:  state,
				}

				event := EventRecovered
				if crash || anr {
					event = EventDown
				}

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
