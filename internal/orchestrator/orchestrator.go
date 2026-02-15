package orchestrator

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/google/uuid"
)

type Orchestrator struct {
	experiments map[string]*models.Experiment
	mu          sync.Mutex
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		experiments: make(map[string]*models.Experiment),
	}
}

func (o *Orchestrator) StartExperiment(faultType, target string, duration int) (*models.Experiment, error) {

	if duration <= 0 {
		return nil, errors.New("duration must be greater than 0")
	}

	if faultType == "" || target == "" {
		return nil, errors.New("faultType and target are required")
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	id := uuid.New().String()

	exp := &models.Experiment{
		ID:        id,
		FaultType: faultType,
		Target:    target,
		Duration:  duration,
		State:     models.StateCreated,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	o.experiments[id] = exp

	go o.runLifecycle(id)

	return exp, nil
}

func (o *Orchestrator) runLifecycle(id string) {
	o.updateState(id, models.StateFaulted)

	time.Sleep(5 * time.Second)

	o.updateState(id, models.StateRecovering)

	time.Sleep(2 * time.Second)

	o.updateState(id, models.StateCompleted)
}

func (o *Orchestrator) updateState(id string, newState models.ExperimentState) {

	o.mu.Lock()
	defer o.mu.Unlock()

	exp, exists := o.experiments[id]
	if !exists {
		return
	}

	exp.State = newState
	exp.UpdatedAt = time.Now()

	fmt.Println("Experiment", id, "state updated to", newState)
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
