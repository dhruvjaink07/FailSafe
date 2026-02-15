package orchestrator

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
	"github.com/google/uuid"
)

// Orchestrator manages experiment lifecycle and coordination.
// It is the control plane of the system
type Orchestrator struct {
	experiments map[string]*models.Experiment  // In-memory experiment store
	monitor     map[string]*monitoring.Monitor // Monitoring engine
	mu          sync.Mutex                     // Protects concurrent access
}

// Initiates the control plane
// It prepares the experiment store and monitoring engine
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		experiments: make(map[string]*models.Experiment),
		monitor:     make(map[string]*monitoring.Monitor),
	}
}

// StartExperiment creates and registers a new experiment.
// It also launches monitoring and lifecycle simulation
func (o *Orchestrator) StartExperiment(faultType, target string, duration int) (*models.Experiment, error) {

	// Basic validation
	if duration <= 0 {
		return nil, errors.New("duration must be greater than 0")
	}

	if faultType == "" || target == "" {
		return nil, errors.New("faultType and target are required")
	}
	// Lock to prevent race condition while modifying map.
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
	// Store experiment in memory
	o.experiments[id] = exp

	// Create monitor for this experiment
	monitor := monitoring.NewMonitor()
	o.monitor[id] = monitor
	monitor.Start(id) // Start monitoring in background

	go o.runLifecycle(id) // Start Lifecycle simulation in background

	return exp, nil
}

// runLifecycle simulates state transitions.
func (o *Orchestrator) runLifecycle(id string) {

	o.updateState(id, models.StateFaulted)

	o.mu.Lock()
	exp := o.experiments[id]
	o.mu.Unlock()

	time.Sleep(time.Duration(exp.Duration) * time.Second)

	o.updateState(id, models.StateRecovering)

	time.Sleep(2 * time.Second)

	o.updateState(id, models.StateCompleted)
}

// updateState safely trasitions experiment state
// Mutex ensures thread-safe update.
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

	if newState == models.StateCompleted {
		if monitor, ok := o.monitor[id]; ok {
			monitor.Stop()
			delete(o.monitor, id)
		}
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
