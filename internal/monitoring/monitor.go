package monitoring

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/shirou/gopsutil/v3/cpu"
)

type EventType string

const (
	EventNone      EventType = ""
	EventDown      EventType = "down"
	EventRecovered EventType = "recovered"
)

type EventCallback func(event EventType, sample models.MetricSample)

type Monitor struct {
	stopChan       chan struct{}
	consecutiveErr int
	isDown         bool
	callback       EventCallback
	dockerManager  *docker.Manager
	containerName  string
}

func NewMonitor(cb EventCallback, dm *docker.Manager, container string) *Monitor {
	return &Monitor{
		stopChan:      make(chan struct{}),
		callback:      cb,
		dockerManager: dm,
		containerName: container,
	}
}

func (m *Monitor) Start(experimentID string, targetURL string) {
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				m.collect(experimentID, targetURL)
			case <-m.stopChan:
				ticker.Stop()
				fmt.Println("Monitoring stopped for", experimentID)
				return
			}
		}
	}()
}

func (m *Monitor) collect(experimentID string, targetURL string) {

	// ---------- HOST CPU ----------
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		fmt.Println("CPU error:", err)
		return
	}

	// ---------- HTTP PROBE ----------
	client := http.Client{Timeout: 2 * time.Second}

	start := time.Now()
	resp, err := client.Get(targetURL)
	latency := time.Since(start)

	statusCode := 0
	success := false

	if err != nil {
		m.consecutiveErr++
	} else {
		statusCode = resp.StatusCode
		resp.Body.Close()

		if statusCode >= 200 && statusCode < 300 {
			success = true
			m.consecutiveErr = 0
		} else {
			m.consecutiveErr++
		}
	}

	// ---------- CONTAINER STATS ----------
	var containerCPU, containerMemMB, containerMemPct float64
	var netIO, blockIO string

	if m.dockerManager != nil {
		cCPU, cMem, cMemPct, nIO, bIO, err :=
			m.dockerManager.GetContainerStats(m.containerName)

		if err == nil {
			containerCPU = cCPU
			containerMemMB = cMem
			containerMemPct = cMemPct
			netIO = nIO
			blockIO = bIO
		}
	}

	// ---------- STATE TRANSITIONS ----------
	event := EventNone

	if m.consecutiveErr >= 3 && !m.isDown {
		m.isDown = true
		event = EventDown
		fmt.Printf("⚠ Experiment %s DOWN detected\n", experimentID)
	}

	if success && m.isDown {
		m.isDown = false
		event = EventRecovered
		fmt.Printf("✔ Experiment %s RECOVERED\n", experimentID)
	}

	// ---------- METRIC SAMPLE ----------
	sample := models.MetricSample{
		Timestamp: time.Now(),
		CPU:       cpuPercent[0],
		LatencyMs: latency.Milliseconds(),
		Status:    statusCode,
		IsDown:    m.isDown,

		ContainerCPU:     containerCPU,
		ContainerMemMB:   containerMemMB,
		ContainerMemPct:  containerMemPct,
		ContainerNetIO:   netIO,
		ContainerBlockIO: blockIO,
	}

	// ---------- CALLBACK ----------
	if m.callback != nil {
		m.callback(event, sample)
	}

	fmt.Printf(
		"[EXP %s] CPU: %.2f%% | C-CPU: %.2f%% | Mem: %.2fMB | Latency: %v | Status: %d\n",
		experimentID,
		cpuPercent[0],
		containerCPU,
		containerMemMB,
		latency,
		statusCode,
	)
}

func (m *Monitor) Stop() {
	close(m.stopChan)
}
