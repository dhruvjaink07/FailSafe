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
	mu          sync.Mutex
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		experiments: make(map[string]*models.Experiment),
		monitors:    make(map[string]*monitoring.Monitor),
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
	o.mu.Unlock()

	// Event callback from Monitor
	callback := func(event monitoring.EventType) {

		o.mu.Lock()
		defer o.mu.Unlock()

		experiment, exists := o.experiments[id]
		if !exists {
			return
		}

		if experiment.State == models.StateCompleted {
			return
		}

		switch event {
		case monitoring.EventDown:
			experiment.State = models.StateFaulted
			experiment.UpdatedAt = time.Now()

		case monitoring.EventRecovered:
			o.completeExperimentLocked(id)
		}
	}

	monitor := monitoring.NewMonitor(callback)

	o.mu.Lock()
	o.monitors[id] = monitor
	o.mu.Unlock()

	monitor.Start(id, targetURL)

	// Duration cap goroutine
	go func() {
		time.Sleep(time.Duration(duration) * time.Second)

		o.mu.Lock()
		defer o.mu.Unlock()

		experiment, exists := o.experiments[id]
		if !exists {
			return
		}

		if experiment.State != models.StateCompleted {
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
	return exp, nil
}
