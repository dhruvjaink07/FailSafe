package orchestrator

import (
	"fmt"
	"log"
	"sort"
	"strings"
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
	if len(exp.Scenario) > 0 {
		o.runScenarioFaults(id)
		return
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
			o.recordFaultEvent(id, config.Type)
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
		o.recordFaultEvent(id, config.Type)
	}

	o.setPhase(id, models.PhaseRecovering)
	time.Sleep(5 * time.Second)

	o.completeExperiment(id)
}

func (o *Orchestrator) runScenarioFaults(id string) {
	o.setPhase(id, models.PhaseInjecting)

	o.mu.Lock()
	exp := o.experiments[id]
	injector := o.injectors[id]
	monitor := o.monitors[id]
	if exp != nil {
		exp.FaultStartedAt = time.Now()
	}
	o.mu.Unlock()

	if exp == nil || injector == nil {
		o.setPhase(id, models.PhaseRecovering)
		time.Sleep(5 * time.Second)
		o.completeExperiment(id)
		return
	}

	scenario := append([]models.ScheduledFault(nil), exp.Scenario...)
	sort.Slice(scenario, func(i, j int) bool {
		return scenario[i].At < scenario[j].At
	})

	start := exp.FaultStartedAt
	if start.IsZero() {
		start = time.Now()
	}

	for _, scheduled := range scenario {
		target := start.Add(time.Duration(scheduled.At) * time.Second)
		if wait := time.Until(target); wait > 0 {
			time.Sleep(wait)
		}

		if scheduled.Trigger != nil && strings.EqualFold(scheduled.Trigger.Type, "request") {
			o.waitForRequestTrigger(id, scheduled.Trigger, time.Now())
		}

		intensity := scheduled.Intensity
		if intensity <= 0 {
			intensity = exp.Intensity
		}

		dur := scheduled.DurationSeconds
		if dur <= 0 {
			dur = 1
		}

		if monitor != nil {
			monitor.SetIntensity(intensity)
		}

		o.mu.Lock()
		exp.CurrentIntensity = intensity
		o.mu.Unlock()

		config := fault.FaultConfig{
			ExperimentID:    id,
			Targets:         exp.Targets,
			TargetType:      exp.TargetType,
			Type:            fault.FaultType(scheduled.Type),
			DurationSeconds: dur,
			Intensity:       intensity,
		}

		// Guard each step so one blocked injector call cannot freeze the experiment in injecting.
		stepTimeout := time.Duration(dur+8) * time.Second
		if stepTimeout < 12*time.Second {
			stepTimeout = 12 * time.Second
		}

		if err := o.injectWithTimeout(injector, config, stepTimeout); err != nil {
			log.Printf("scenario step timeout/error id=%s type=%s at=%ds: %v", id, config.Type, scheduled.At, err)
			continue
		}

		o.recordFaultEvent(id, config.Type)
	}

	o.setPhase(id, models.PhaseRecovering)
	time.Sleep(5 * time.Second)
	o.completeExperiment(id)
}

func (o *Orchestrator) injectWithTimeout(injector fault.Injector, config fault.FaultConfig, timeout time.Duration) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- injector.Inject(config)
	}()

	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("inject timed out after %s", timeout)
	}
}

func (o *Orchestrator) waitForRequestTrigger(id string, trigger *models.FaultTrigger, since time.Time) bool {
	if trigger == nil {
		return false
	}

	timeout := trigger.TimeoutSeconds
	if timeout <= 0 {
		timeout = 12
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for time.Now().Before(deadline) {
		if o.hasRequestSignal(id, trigger.Pattern, since) {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}

	return false
}

func (o *Orchestrator) hasRequestSignal(id, pattern string, since time.Time) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	endpointSamples, ok := o.metrics[id]
	if !ok {
		return false
	}

	lowerPattern := strings.ToLower(strings.TrimSpace(pattern))

	for _, samples := range endpointSamples {
		for _, sample := range samples {
			if sample.Timestamp.Before(since) {
				continue
			}
			if strings.TrimSpace(sample.AppEvent) == "" {
				continue
			}
			if lowerPattern == "" || strings.Contains(strings.ToLower(sample.AppEvent), lowerPattern) {
				return true
			}
		}
	}

	return false
}
