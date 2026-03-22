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
	EventDegraded  EventType = "degraded" // NEW
)

type EventCallback func(event EventType, sample models.MetricSample)

type Monitor struct {
	stopChan chan struct{}

	consecutiveErr map[string]int
	isDown         map[string]bool

	callback EventCallback

	dockerManager *docker.Manager
	containers    []string

	CurrentIntensity int
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

func (m *Monitor) SetIntensity(i int) {
	m.CurrentIntensity = i
}

func (m *Monitor) Start(experimentID string, endpoints []string) {

	ticker := time.NewTicker(500 * time.Millisecond) // IMPROVED PRECISION

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

	hostCPU := 0.0
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		hostCPU = cpuPercent[0]
	}

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

	client := &http.Client{
		Timeout: 500 * time.Millisecond,
	}

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
			Intensity:           m.CurrentIntensity,
		}

		// always emit sample
		if m.callback != nil {
			m.callback(EventType("sample"), sample)
		}

		// -------- ADAPTIVE DEGRADATION DETECTION --------

		// dynamic threshold based on baseline
		baseline := m.getBaselineP95(url)
		threshold := int(float64(baseline) * 2.0)

		// safeguard for low-latency systems (like nginx)
		if threshold < 20 {
			threshold = 20
		}

		latencyDegraded := latency.Milliseconds() > int64(threshold)
		errorDegraded := m.consecutiveErr[url] == 1

		if latencyDegraded || errorDegraded {
			if m.callback != nil {
				m.callback(EventDegraded, sample)
			}
		}

		// -------- DOWN DETECTION --------

		if m.consecutiveErr[url] >= 3 && !m.isDown[url] {
			m.isDown[url] = true
			if m.callback != nil {
				m.callback(EventDown, sample)
			}
		}

		// -------- RECOVERY DETECTION --------

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

func (m *Monitor) getBaselineP95(endpoint string) int64 {
	// fallback static baseline for now
	// orchestrator already computes real baseline
	return 5
}
