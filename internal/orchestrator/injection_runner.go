package orchestrator

import (
	"time"

	"github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) runTimeline(id string) {
	o.setPhase(id, models.PhaseBaseline)
	time.Sleep(6 * time.Second)
	o.computeBaseline(id)

	o.mu.Lock()
	exp := o.experiments[id]
	o.mu.Unlock()
	if o.db != nil {
		_ = o.db.UpdateBaseline(exp)
	}
	if !exp.Adaptive {
		o.runStaticFault(id)
		return
	}

	o.setPhase(id, models.PhaseInjecting)

	low := 0
	high := exp.MaxIntensity
	maxStable := 0
	breaking := 0

	for low <= high {
		mid := (low + high) / 2
		startTime := time.Now()

		o.mu.Lock()

		// Reset timeline points for the current intensity attempt.
		o.firstImpact[id] = make(map[string]time.Time)
		o.recoveryAt[id] = make(map[string]time.Time)
		delete(o.downtime, id)

		exp = o.experiments[id]

		exp.CurrentIntensity = mid
		exp.IntensityHistory = append(exp.IntensityHistory, mid)
		exp.FaultStartedAt = startTime

		if monitor, ok := o.monitors[id]; ok {
			monitor.SetIntensity(mid)
		}

		o.mu.Unlock()

		config := fault.FaultConfig{
			ExperimentID:    id,
			Targets:         exp.Targets,
			TargetType:      exp.TargetType,
			Type:            fault.FaultType(exp.FaultType),
			DurationSeconds: 5,
			Intensity:       mid,
		}

		if injector, ok := o.injectors[id]; ok {
			_ = injector.Inject(config)
		}
		time.Sleep(10 * time.Second)

		o.mu.Lock()

		firstImpactCopy := make(map[string]time.Time)
		for k, v := range o.firstImpact[id] {
			firstImpactCopy[k] = v
		}

		recoveryCopy := make(map[string]time.Time)
		for k, v := range o.recoveryAt[id] {
			recoveryCopy[k] = v
		}

		exp.TimelineHistory[mid] = models.IntensityTimeline{
			FaultStartedAt: startTime,
			FirstImpact:    firstImpactCopy,
			RecoveryAt:     recoveryCopy,
		}

		o.mu.Unlock()

		degraded := o.isExperimentDegraded(id, startTime)
		degradedMap := o.getDegradedEndpoints(id, startTime)

		o.mu.Lock()
		exp = o.experiments[id]
		o.mu.Unlock()

		blast, depth := o.computeGraphImpactScore(exp, degradedMap)

		if degraded {
			breaking = mid
			high = mid - 1
			continue
		}

		if blast > 50 || depth >= 2 {
			breaking = mid
			high = mid - 1
			continue
		}

		maxStable = mid
		low = mid + 1
	}

	o.mu.Lock()
	exp.MaxStableIntensity = maxStable
	exp.BreakingIntensity = breaking
	o.mu.Unlock()

	o.setPhase(id, models.PhaseRecovering)
	time.Sleep(5 * time.Second)

	o.completeExperiment(id)
}

func (o *Orchestrator) runStaticFault(id string) {
	o.setPhase(id, models.PhaseInjecting)

	o.mu.Lock()
	exp := o.experiments[id]
	exp.FaultStartedAt = time.Now()
	o.mu.Unlock()

	config := fault.FaultConfig{
		ExperimentID:    id,
		Targets:         exp.Targets,
		TargetType:      exp.TargetType,
		Type:            fault.FaultType(exp.FaultType),
		DurationSeconds: exp.Duration / 2,
		Intensity:       exp.Intensity,
	}

	if injector, ok := o.injectors[id]; ok {
		_ = injector.Inject(config)
	}

	o.setPhase(id, models.PhaseRecovering)
	time.Sleep(5 * time.Second)

	o.completeExperiment(id)
}
