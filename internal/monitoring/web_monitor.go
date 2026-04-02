package monitoring

import (
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

// WebMonitor tracks browser-pushed frontend samples.
// Unlike backend monitors, it does not poll endpoints; it watches ingest freshness.
type WebMonitor struct {
	callback EventCallback

	mu               sync.Mutex
	stop             chan struct{}
	started          bool
	experimentID     string
	currentIntensity int
	lastIngestAt     time.Time
	unhealthy        bool

	staleAfter time.Duration
	tickerStep time.Duration
}

func NewWebMonitor(cb EventCallback) *WebMonitor {
	return &WebMonitor{
		callback:   cb,
		stop:       make(chan struct{}),
		staleAfter: 6 * time.Second,
		tickerStep: 2 * time.Second,
	}
}

func (m *WebMonitor) Start(experimentID string, _ []string) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.experimentID = experimentID
	m.lastIngestAt = time.Now()
	m.stop = make(chan struct{})
	stop := m.stop
	step := m.tickerStep
	m.mu.Unlock()

	ticker := time.NewTicker(step)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.checkStaleness()
			case <-stop:
				return
			}
		}
	}()
}

func (m *WebMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return
	}
	m.started = false
	close(m.stop)
}

func (m *WebMonitor) SetIntensity(i int) {
	m.mu.Lock()
	m.currentIntensity = i
	m.mu.Unlock()
}

// RecordIngest is called by orchestrator when frontend metric batches arrive.
func (m *WebMonitor) RecordIngest(sample models.FrontendMetrics) {
	m.mu.Lock()
	m.lastIngestAt = time.Now()
	unhealthy := m.unhealthy
	m.unhealthy = false
	intensity := m.currentIntensity
	callback := m.callback
	m.mu.Unlock()

	if callback == nil {
		return
	}

	metric := models.MetricSample{
		Timestamp: time.Now(),
		Endpoint:  normalizePage(sample.Page),
		Intensity: intensity,
		LatencyMs: int64(sample.Metrics.INP),
		Status:    200,
		Warning:   sample.Metrics.Errors > 0 || sample.Metrics.UnhandledRejections > 0,
	}

	if unhealthy {
		callback(EventRecovered, metric)
		return
	}
	callback(EventType("sample"), metric)
}

func (m *WebMonitor) checkStaleness() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	stale := time.Since(m.lastIngestAt) > m.staleAfter
	alreadyUnhealthy := m.unhealthy
	if stale {
		m.unhealthy = true
	}
	intensity := m.currentIntensity
	callback := m.callback
	m.mu.Unlock()

	if callback == nil || !stale || alreadyUnhealthy {
		return
	}

	callback(EventDegraded, models.MetricSample{
		Timestamp: time.Now(),
		Endpoint:  "frontend:ingest",
		Intensity: intensity,
		Status:    408,
		Warning:   true,
	})
}

func normalizePage(page string) string {
	if page == "" {
		return "/"
	}
	return page
}
