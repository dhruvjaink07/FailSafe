package monitoring

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

// Monitor represents a runtime metric collector
// It runs in background and samples system metrics periodically until stopped.=
type Monitor struct {
	// stopChan is used to signal the monitoring goroutine to stop safely
	// closing this channel tells the loop to terminate
	stopChan chan struct{}
}

// NewMonitor creates a new monitoring instance.
// Each monitor should ideally be tied to a single experiment.
func NewMonitor() *Monitor {
	return &Monitor{
		stopChan: make(chan struct{}),
	}
}

// Start begins metric collection for a specific experiment.
// It launches a goroutine that runs independently from the main thread.
func (m *Monitor) Start(experimentID string) {

	// ticker triggers an event every 1 second
	// This avoids using  time.Sleep inside a loop and allows for responsive shutdown
	ticker := time.NewTicker(1 * time.Second)

	// Launch background goroutine.
	go func() {
		for {
			select {

			// Case 1: Every 1 second, collect metrics
			case <-ticker.C:
				m.collect(experimentID)
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
func (m *Monitor) collect(experimentID string) {

	// cpu.Percent returns current CPU usage.
	// First argument: interval (0 means instant snapshot)
	// Second argument: perCPU (false = overall CPU usage, true = usage per core)
	percent, err := cpu.Percent(0, false)
	if err != nil {
		fmt.Println("CPU error: ", err)
		return
	}

	// percent is a slice, we take the first value since we requested overall CPU usage
	fmt.Printf("Experiment %s CPU Usage: %.2f%%\n", experimentID, percent[0])
}

// Stop signals the monitoring goroutine to terminate.
// Closing the channel causes the select case to trigger.
func (m *Monitor) Stop() {
	close(m.stopChan)
}
