package orchestrator

import (
	"errors"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) createExperiment(
	id string,
	targets []string,
	targetType string,
	observationType string,
	faultType string,
	duration int,
	adaptive bool,
	stepIntensity int,
	maxIntensity int,
	deps models.DependencyGraph,
	targetMap map[string][]string,
) *models.Experiment {
	return &models.Experiment{
		ID:              id,
		Targets:         targets,
		TargetType:      targetType,
		ObservationType: observationType,
		FaultType:       faultType,
		Duration:        duration,
		State:           models.StateRunning,
		Phase:           models.PhaseBaseline,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),

		Adaptive:          adaptive,
		StepIntensity:     stepIntensity,
		MaxIntensity:      maxIntensity,
		DependencyGraph:   deps,
		TargetEndpointMap: targetMap,
		TimelineHistory:   make(map[int]models.IntensityTimeline),
	}
}

func (o *Orchestrator) registerExperiment(id string, exp *models.Experiment, observedEndpoints []string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	exp.ObservedEndpoints = observedEndpoints
	if exp.DependencyGraph != nil {
		exp.GraphMetadata = computeGraphMeta(exp.DependencyGraph)
	}

	o.experiments[id] = exp
	o.metrics[id] = make(map[string][]models.MetricSample)
	o.metricBuffer[id] = make([]models.MetricSample, 0, 64)
	o.firstImpact[id] = make(map[string]time.Time)
	o.recoveryAt[id] = make(map[string]time.Time)
	o.totalDown[id] = 0
	o.failures[id] = 0
	o.lastRecovery[id] = 0

	for _, ep := range observedEndpoints {
		o.metrics[id][ep] = []models.MetricSample{}
	}

	if o.db != nil {
		_ = o.db.InsertExperiment(exp)
	}
}

func (o *Orchestrator) setPhase(id string, phase models.ExperimentPhase) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if exp, ok := o.experiments[id]; ok {
		exp.Phase = phase
		exp.UpdatedAt = time.Now()
	}
}

func (o *Orchestrator) completeExperiment(id string) {
	o.mu.Lock()
	exp, ok := o.experiments[id]
	if !ok {
		o.mu.Unlock()
		return
	}

	exp.State = models.StateCompleted
	exp.Phase = models.PhaseCompleted
	exp.UpdatedAt = time.Now()

	monitor := o.monitors[id]
	o.mu.Unlock()

	if monitor != nil {
		monitor.Stop()
	}

	if o.db != nil {
		_ = o.flushMetricsBatch(id)
		_ = o.db.UpdateExperimentResults(exp)

		if metrics, err := o.GetMetrics(id); err == nil {
			if data, ok := metrics.(map[string]interface{}); ok {
				_ = o.db.InsertAggregatedMetrics(id, data)
				_ = o.db.InsertExperimentSummary(id, data)
			}
		}
	}
}

func (o *Orchestrator) GetExperiment(id string) (*models.Experiment, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	exp, ok := o.experiments[id]
	if !ok {
		return nil, errors.New("experiment not found")
	}

	return exp, nil
}

func (o *Orchestrator) StopExperiment(id string) error {
	o.mu.Lock()
	exp, ok := o.experiments[id]
	if !ok {
		o.mu.Unlock()
		return errors.New("experiment not found")
	}

	monitor := o.monitors[id]
	exp.State = models.StateFailed
	exp.Phase = models.PhaseCompleted
	exp.UpdatedAt = time.Now()
	o.mu.Unlock()

	if monitor != nil {
		monitor.Stop()
	}

	if o.db != nil {
		_ = o.flushMetricsBatch(id)
		_ = o.db.UpdateExperimentResults(exp)
	}

	return nil
}
