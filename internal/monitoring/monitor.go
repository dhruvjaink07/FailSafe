package monitoring

import (
	"net/http"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/shirou/gopsutil/v3/cpu"
)

type EventType string

const (
	EventDown      EventType = "down"
	EventRecovered EventType = "recovered"
)

type EventCallback func(event EventType, sample models.MetricSample)

type Monitor struct {
	stopChan chan struct{}

	// Track failure streak per endpoint
	consecutiveErr map[string]int
	isDown         map[string]bool

	callback EventCallback

	dockerManager *docker.Manager
	containers    []string
}

func NewMonitor(
	cb EventCallback,
	dm *docker.Manager,
	containers []string,
) *Monitor {

	return &Monitor{
		stopChan:       make(chan struct{}),
		callback:       cb,
		dockerManager:  dm,
		containers:     containers,
		consecutiveErr: make(map[string]int),
		isDown:         make(map[string]bool),
	}
}

func (m *Monitor) Start(experimentID string, endpoints []string) {

	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {

			case <-ticker.C:
				m.collect(experimentID, endpoints)

			case <-m.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

func (m *Monitor) collect(experimentID string, endpoints []string) {

	// ---------------- HOST CPU ----------------

	hostCPU := 0.0
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		hostCPU = cpuPercent[0]
	}

	// ---------------- CONTAINER STATS ----------------

	var totalContainerCPU float64
	var totalMem float64
	var totalMemPercent float64
	var totalNetIO string
	var totalBlockIO string

	for _, container := range m.containers {

		stats, err := m.dockerManager.GetContainerStats(container)
		if err != nil {
			continue
		}

		totalContainerCPU += stats.CPUPercent
		totalMem += stats.MemoryMB
		totalMemPercent += stats.MemoryPercent
		totalNetIO = stats.NetworkIO
		totalBlockIO = stats.BlockIO
	}

	containerCount := float64(len(m.containers))
	if containerCount > 0 {
		totalContainerCPU /= containerCount
		totalMem /= containerCount
		totalMemPercent /= containerCount
	}

	client := http.Client{Timeout: 2 * time.Second}

	// ---------------- PER-ENDPOINT MONITORING ----------------

	for _, url := range endpoints {

		start := time.Now()
		resp, err := client.Get(url)
		latency := time.Since(start)

		statusCode := 0
		success := false

		if err != nil {
			m.consecutiveErr[url]++
		} else {
			statusCode = resp.StatusCode
			resp.Body.Close()

			if statusCode >= 200 && statusCode < 300 {
				success = true
				m.consecutiveErr[url] = 0
			} else {
				m.consecutiveErr[url]++
			}
		}

		sample := models.MetricSample{
			Timestamp:           time.Now(),
			Endpoint:            url,
			CPU:                 hostCPU,
			LatencyMs:           latency.Milliseconds(),
			Status:              statusCode,
			IsDown:              m.isDown[url],
			ContainerCPU:        totalContainerCPU,
			ContainerMemoryMB:   totalMem,
			ContainerMemPercent: totalMemPercent,
			ContainerNetIO:      totalNetIO,
			ContainerBlockIO:    totalBlockIO,
		}

		// Emit sample
		if m.callback != nil {
			m.callback("", sample)
		}

		// -------- DOWN DETECTION PER ENDPOINT --------

		if m.consecutiveErr[url] >= 3 && !m.isDown[url] {
			m.isDown[url] = true
			if m.callback != nil {
				m.callback(EventDown, sample)
			}
		}

		// -------- RECOVERY DETECTION PER ENDPOINT --------

		if success && m.isDown[url] {
			m.isDown[url] = false
			if m.callback != nil {
				m.callback(EventRecovered, sample)
			}
		}
	}
}

func (m *Monitor) Stop() {
	close(m.stopChan)
}
