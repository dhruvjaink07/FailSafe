package orchestrator

import (
	"errors"
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
	"github.com/google/uuid"
)

type Orchestrator struct {
	experiments map[string]*models.Experiment
	monitors    map[string]*monitoring.Monitor
	metrics     map[string][]models.MetricSample
	downtime    map[string]time.Time
	totalDown   map[string]time.Duration
	mu          sync.Mutex
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		experiments: make(map[string]*models.Experiment),
		monitors:    make(map[string]*monitoring.Monitor),
		metrics:     make(map[string][]models.MetricSample),
		downtime:    make(map[string]time.Time),
		totalDown:   make(map[string]time.Duration),
	}
}

func (o *Orchestrator) StartExperiment(
	faultType, target, targetURL string,
	duration int,
) (*models.Experiment, error) {

	if duration <= 0 {
		return nil, errors.New("duration must be greater than 0")
	}

	if faultType == "" || target == "" || targetURL == "" {
		return nil, errors.New("faultType, target and targetURL are required")
	}

	id := uuid.New().String()

	exp := &models.Experiment{
		ID:        id,
		FaultType: faultType,
		Target:    target,
		TargetURL: targetURL,
		Duration:  duration,
		State:     models.StateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	o.mu.Lock()
	o.experiments[id] = exp
	o.metrics[id] = []models.MetricSample{}
	o.totalDown[id] = 0
	o.mu.Unlock()

	callback := func(event monitoring.EventType, sample models.MetricSample) {

		o.mu.Lock()
		defer o.mu.Unlock()

		// Store metric sample
		o.metrics[id] = append(o.metrics[id], sample)

		experiment, exists := o.experiments[id]
		if !exists {
			return
		}

		if experiment.State == models.StateCompleted {
			return
		}

		switch event {

		case monitoring.EventDown:
			o.downtime[id] = time.Now()
			experiment.State = models.StateFaulted
			experiment.UpdatedAt = time.Now()

		case monitoring.EventRecovered:
			if start, ok := o.downtime[id]; ok {
				o.totalDown[id] += time.Since(start)
				delete(o.downtime, id)
			}
			o.completeExperimentLocked(id)
		}
	}

	monitor := monitoring.NewMonitor(callback)

	o.mu.Lock()
	o.monitors[id] = monitor
	o.mu.Unlock()

	monitor.Start(id, targetURL)

	// Duration cap
	go func() {
		time.Sleep(time.Duration(duration) * time.Second)

		o.mu.Lock()
		defer o.mu.Unlock()

		experiment, exists := o.experiments[id]
		if !exists {
			return
		}

		if experiment.State != models.StateCompleted {

			// If currently down, accumulate downtime
			if start, ok := o.downtime[id]; ok {
				o.totalDown[id] += time.Since(start)
				delete(o.downtime, id)
			}

			o.completeExperimentLocked(id)
		}
	}()

	return exp, nil
}

// StopExperiment allows manual stopping
func (o *Orchestrator) StopExperiment(id string) error {

	o.mu.Lock()
	defer o.mu.Unlock()

	experiment, exists := o.experiments[id]
	if !exists {
		return errors.New("experiment not found")
	}

	if experiment.State == models.StateCompleted {
		return nil
	}

	o.completeExperimentLocked(id)
	return nil
}

// completeExperimentLocked assumes mutex already locked
func (o *Orchestrator) completeExperimentLocked(id string) {

	experiment := o.experiments[id]
	experiment.State = models.StateCompleted
	experiment.UpdatedAt = time.Now()

	if monitor, ok := o.monitors[id]; ok {
		monitor.Stop()
		delete(o.monitors, id)
	}
}

func (o *Orchestrator) GetExperiment(id string) (*models.Experiment, error) {

	o.mu.Lock()
	defer o.mu.Unlock()

	exp, exists := o.experiments[id]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	// Return a copy to avoid external mutation
	copyExp := *exp
	return &copyExp, nil
}

func (o *Orchestrator) GetMetrics(id string) (interface{}, error) {

	o.mu.Lock()
	defer o.mu.Unlock()

	samples, exists := o.metrics[id]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	totalSamples := len(samples)
	if totalSamples == 0 {
		return nil, errors.New("no metrics collected")
	}

	var totalLatency int64
	var errorCount int

	for _, s := range samples {
		totalLatency += s.LatencyMs
		if s.Status >= 400 {
			errorCount++
		}
	}

	avgLatency := totalLatency / int64(totalSamples)

	totalDuration := time.Since(o.experiments[id].CreatedAt)

	resilienceScore := 1.0
	if totalDuration > 0 {
		resilienceScore = 1 - (o.totalDown[id].Seconds() / totalDuration.Seconds())
	}
	return map[string]interface{}{
		"total_samples":    totalSamples,
		"average_latency":  avgLatency,
		"error_count":      errorCount,
		"total_downtime_s": o.totalDown[id].Seconds(),
		"resilience_score": resilienceScore,
		"samples":          samples,
	}, nil
}
