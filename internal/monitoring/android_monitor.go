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
	return m.appLifecycleState() != "not_running"
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
				hasWarning := false
				crashReason := ""
				crashThread := ""
				crashClass := ""
				crashSignature := ""
				requestEvent := ""

				for _, ev := range events {
					if ev.Type == "crash" {
						crash = true
						if crashReason == "" {
							crashReason = ev.Message
							crashThread = ev.Thread
							crashClass, crashSignature = classifyCrash(ev.Message)
						}
					}
					if ev.Type == "anr" {
						anr = true
					}
					if ev.Type == "request" && requestEvent == "" {
						requestEvent = ev.Message
					}
					if ev.Type == "warning" {
						hasWarning = true
					}
				}

				if !anr {
					anr = m.HasANR()
				}
				state := m.appLifecycleState()
				if crash {
					state = "crash"
				} else if anr {
					state = "anr"
				} else if hasWarning && state == "running" {
					state = "degraded"
				}

				sample := models.MetricSample{
					Endpoint:       m.pkg,
					Timestamp:      time.Now(),
					Crash:          crash,
					CrashReason:    crashReason,
					CrashThread:    crashThread,
					CrashClass:     crashClass,
					CrashSignature: crashSignature,
					AppEvent:       requestEvent,
					Warning:        hasWarning,
					ANR:            anr,
					AppState:       state,
					Intensity:      m.currentIntensity,
				}

				event := EventType("sample")

				switch state {
				case "not_running", "crash", "anr":
					event = EventDown
					m.unhealthyActive = true
					m.healthyStreak = 0
				case "background", "degraded":
					event = EventDegraded
					m.unhealthyActive = true
					m.healthyStreak = 0
				default:
					if m.unhealthyActive {
						m.healthyStreak++
						if m.healthyStreak >= 1 {
							event = EventRecovered
							m.unhealthyActive = false
							m.healthyStreak = 0
						}
					}
				}

				m.previousState = state

				m.callback(event, sample)

			case <-m.stop:
				return
			}
		}
	}()
}

func (m *AndroidMonitor) appLifecycleState() string {
	if _, ok := m.getAppPID(); !ok {
		return "not_running"
	}

	out, err := m.adb.Dumpsys("activity activities")
	if err != nil {
		return "running"
	}

	lower := strings.ToLower(out)
	pkgLower := strings.ToLower(m.pkg)

	resumedHints := []string{"mresumedactivity", "topresumedactivity", "resumedactivity"}
	for _, hint := range resumedHints {
		idx := strings.Index(lower, hint)
		if idx == -1 {
			continue
		}
		end := idx + 300
		if end > len(lower) {
			end = len(lower)
		}
		if strings.Contains(lower[idx:end], pkgLower) {
			return "running"
		}
	}

	if strings.Contains(lower, pkgLower) {
		return "background"
	}

	return "not_running"
}

func (m *AndroidMonitor) Stop() {
	close(m.stop)
}

func classifyCrash(message string) (string, string) {
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "nullpointerexception"):
		return "ui_bug", "NullPointerException"
	case strings.Contains(lower, "sockettimeoutexception"):
		return "network_bug", "SocketTimeoutException"
	case strings.Contains(lower, "timeoutexception"):
		return "network_bug", "TimeoutException"
	case strings.Contains(lower, "illegalstateexception"):
		return "lifecycle_bug", "IllegalStateException"
	default:
		if strings.TrimSpace(message) == "" {
			return "unknown", "unknown"
		}
		return "unknown", "unclassified"
	}
}
