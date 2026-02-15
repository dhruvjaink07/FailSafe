package monitoring

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/shirou/gopsutil/v3/cpu"
)

// Event types
type EventType string

const (
	EventDown      EventType = "down"
	EventRecovered EventType = "recovered"
)

// EventCallback allows monitor to notify orchestrator
type EventCallback func(event EventType, sample models.MetricSample)

// Monitor represents a runtime metric collector
// It runs in background and samples system metrics periodically until stopped.=
type Monitor struct {
	// stopChan is used to signal the monitoring goroutine to stop safely
	// closing this channel tells the loop to terminate
	stopChan       chan struct{}
	consecutiveErr int
	isDown         bool
	callback       EventCallback
}

// NewMonitor creates a new monitoring instance.
// Each monitor should ideally be tied to a single experiment.
func NewMonitor(callBack EventCallback) *Monitor {
	return &Monitor{
		stopChan: make(chan struct{}),
		callback: callBack,
	}
}

// Start begins metric collection for a specific experiment.
// It launches a goroutine that runs independently from the main thread.
func (m *Monitor) Start(experimentID string, targetURL string) {

	// ticker triggers an event every 1 second
	// This avoids using  time.Sleep inside a loop and allows for responsive shutdown
	ticker := time.NewTicker(1 * time.Second)

	// Launch background goroutine.
	go func() {
		for {
			select {

			// Case 1: Every 1 second, collect metrics
			case <-ticker.C:
				m.collect(experimentID, targetURL)
			// Case 2: If stop signal is received, clean up and exit
			case <-m.stopChan:
				ticker.Stop() // Always stop ticker to prevent memory leaks
				fmt.Println("Monitoring stopped for", experimentID)
				return
			}
		}
	}()
}

// collect is responsible for fetching runtime metrics
// Right now it collects CPU usage only
func (m *Monitor) collect(experimentID string, targetURL string) {

	// -------- CPU METRIC --------
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		fmt.Println("CPU error:", err)
		return
	}

	// -------- HTTP CLIENT WITH TIMEOUT --------
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	start := time.Now()
	resp, err := client.Get(targetURL)
	latency := time.Since(start)

	statusCode := 0
	success := false

	if err != nil {
		// Network failure (timeout, refused connection, etc.)
		m.consecutiveErr++
	} else {
		statusCode = resp.StatusCode
		resp.Body.Close()

		// Only 2xx is considered healthy
		if statusCode >= 200 && statusCode < 300 {
			success = true
			m.consecutiveErr = 0
		} else {
			m.consecutiveErr++
		}
	}

	sample := models.MetricSample{
		Timestamp: time.Now(),
		CPU:       cpuPercent[0],
		LatencyMs: latency.Milliseconds(),
		Status:    statusCode,
		IsDown:    m.isDown,
	}

	if m.callback != nil {
		m.callback("", sample)
	}
	// -------- DEBUG ERROR COUNT --------
	fmt.Printf("Consecutive Errors: %d\n", m.consecutiveErr)

	// -------- DOWN DETECTION --------
	if m.consecutiveErr >= 3 && !m.isDown {
		m.isDown = true
		fmt.Printf("⚠ Experiment %s DOWN detected\n", experimentID)

		if m.callback != nil {
			m.callback(EventDown, sample)
		}
	}

	// -------- RECOVERY DETECTION --------
	if success && m.isDown {
		m.isDown = false
		fmt.Printf("✔ Experiment %s RECOVERED\n", experimentID)
		if m.callback != nil {
			m.callback(EventRecovered, sample)
		}
	}

	// -------- METRIC OUTPUT --------
	fmt.Printf(
		"[EXP %s] CPU: %.2f%% | Latency: %v | Status: %d\n",
		experimentID,
		cpuPercent[0],
		latency,
		statusCode,
	)
}

// Stop signals the monitoring goroutine to terminate.
// Closing the channel causes the select case to trigger.
func (m *Monitor) Stop() {
	close(m.stopChan)
}
