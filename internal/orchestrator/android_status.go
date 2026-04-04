package orchestrator

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) GetAndroidStatus(id string) (map[string]interface{}, error) {
	o.mu.Lock()
	exp, ok := o.experiments[id]
	if !ok {
		o.mu.Unlock()
		if o.db == nil {
			return nil, errors.New("experiment not found")
		}

		targetType, err := o.db.GetExperimentTargetType(id)
		if err != nil {
			return nil, err
		}
		if targetType == "" {
			return nil, errors.New("experiment not found")
		}
		if targetType != "android" {
			return nil, errors.New("experiment is not android")
		}

		payload, err := o.db.GetPlatformStatusPayload(targetType, id)
		if err != nil {
			return nil, err
		}
		if payload == nil {
			return nil, errors.New("no status found")
		}
		return payload, nil
	}

	if exp.TargetType != string(models.TargetAndroid) && exp.ObservationType != "android" {
		o.mu.Unlock()
		return nil, errors.New("experiment is not android")
	}

	endpointSamples := o.metrics[id]
	firstImpact := make(map[string]time.Time)
	for ep, t := range o.firstImpact[id] {
		firstImpact[ep] = t
	}

	recovery := make(map[string]time.Time)
	for ep, t := range o.recoveryAt[id] {
		recovery[ep] = t
	}

	o.mu.Unlock()

	flat := make([]models.MetricSample, 0)
	for _, list := range endpointSamples {
		flat = append(flat, list...)
	}

	sort.Slice(flat, func(i, j int) bool {
		return flat[i].Timestamp.Before(flat[j].Timestamp)
	})
	history := o.getFaultHistory(id)

	now := time.Now()
	elapsedMs := now.Sub(exp.CreatedAt).Milliseconds()
	if elapsedMs < 0 {
		elapsedMs = 0
	}

	phaseElapsedMs := now.Sub(exp.UpdatedAt).Milliseconds()
	if phaseElapsedMs < 0 {
		phaseElapsedMs = 0
	}

	faultElapsedMs := int64(0)
	if !exp.FaultStartedAt.IsZero() {
		faultElapsedMs = now.Sub(exp.FaultStartedAt).Milliseconds()
		if faultElapsedMs < 0 {
			faultElapsedMs = 0
		}
	}

	currentState := "unknown"
	currentSampleAt := time.Time{}
	if len(flat) > 0 {
		last := flat[len(flat)-1]
		currentState = classifyAndroidState(last)
		currentSampleAt = last.Timestamp
	}

	hadImpact := len(firstImpact) > 0
	if !hadImpact && len(flat) > 0 {
		if fallbackImpact, fallbackRecovery := inferAndroidImpactAndRecovery(flat, exp.FaultStartedAt); !fallbackImpact.IsZero() || !fallbackRecovery.IsZero() {
			endpoint := androidPrimaryEndpoint(exp, flat)
			if !fallbackImpact.IsZero() {
				firstImpact[endpoint] = fallbackImpact
				hadImpact = true
			}
			if !fallbackRecovery.IsZero() {
				recovery[endpoint] = fallbackRecovery
			}
			o.mu.Lock()
			if _, ok := o.firstImpact[id]; !ok {
				o.firstImpact[id] = make(map[string]time.Time)
			}
			if _, ok := o.recoveryAt[id]; !ok {
				o.recoveryAt[id] = make(map[string]time.Time)
			}
			if !fallbackImpact.IsZero() {
				o.firstImpact[id][endpoint] = fallbackImpact
			}
			if !fallbackRecovery.IsZero() {
				o.recoveryAt[id][endpoint] = fallbackRecovery
			}
			o.mu.Unlock()
		}
	}

	if (!hadImpact || len(recovery) == 0) && len(history) > 0 {
		synthImpact, synthRecovery := synthesizeAndroidTimeline(exp, history, currentState, currentSampleAt, now)
		if !synthImpact.IsZero() || !synthRecovery.IsZero() {
			endpoint := androidPrimaryEndpoint(exp, flat)
			persist := false
			if !hadImpact && !synthImpact.IsZero() {
				firstImpact[endpoint] = synthImpact
				hadImpact = true
				persist = true
			}
			if len(recovery) == 0 && !synthRecovery.IsZero() {
				recovery[endpoint] = synthRecovery
				persist = true
			}
			if persist {
				o.mu.Lock()
				if _, ok := o.firstImpact[id]; !ok {
					o.firstImpact[id] = make(map[string]time.Time)
				}
				if _, ok := o.recoveryAt[id]; !ok {
					o.recoveryAt[id] = make(map[string]time.Time)
				}
				if ts, ok := firstImpact[endpoint]; ok && !ts.IsZero() {
					o.firstImpact[id][endpoint] = ts
				}
				if ts, ok := recovery[endpoint]; ok && !ts.IsZero() {
					o.recoveryAt[id][endpoint] = ts
				}
				o.mu.Unlock()
			}
		}
	}
	health := "healthy"
	if currentState == "not_running" || currentState == "crash" || currentState == "anr" {
		health = "down"
	} else if hadImpact || currentState == "background" || currentState == "degraded" {
		health = "degraded"
	}

	transitions := make([]map[string]interface{}, 0)
	prev := ""
	for _, s := range flat {
		state := classifyAndroidState(s)
		if prev != "" && prev != state {
			transitions = append(transitions, map[string]interface{}{
				"from": prev,
				"to":   state,
				"at":   s.Timestamp,
			})
		}
		prev = state
	}

	maxTransitions := 20
	if len(transitions) > maxTransitions {
		transitions = transitions[len(transitions)-maxTransitions:]
	}

	faultsApplied := make([]map[string]interface{}, 0, len(history))
	for _, ev := range history {
		eventPhase := exp.Phase
		if strings.TrimSpace(ev.Phase) != "" {
			eventPhase = models.ExperimentPhase(ev.Phase)
		}
		faultsApplied = append(faultsApplied, map[string]interface{}{
			"type":     ev.Type,
			"at":       ev.Timestamp,
			"at_ms":    o.relativeMillis(exp.FaultStartedAt, ev.Timestamp),
			"in_phase": eventPhase,
		})
	}

	totalDurationMs := int64(exp.Duration) * 1000
	progressPercent := 0.0
	if totalDurationMs > 0 {
		progressPercent = float64(elapsedMs) / float64(totalDurationMs) * 100
		if progressPercent < 0 {
			progressPercent = 0
		}
		if progressPercent > 100 {
			progressPercent = 100
		}
	}
	if exp.State == models.StateCompleted || exp.State == models.StateFailed || exp.Phase == models.PhaseCompleted {
		progressPercent = 100
	}

	status := map[string]interface{}{
		"id":                exp.ID,
		"target_type":       exp.TargetType,
		"observation_type":  exp.ObservationType,
		"state":             exp.State,
		"phase":             exp.Phase,
		"server_time":       now,
		"is_terminal":       exp.State == models.StateCompleted || exp.State == models.StateFailed || exp.Phase == models.PhaseCompleted,
		"created_at":        exp.CreatedAt,
		"updated_at":        exp.UpdatedAt,
		"current_state":     currentState,
		"current_sample_at": currentSampleAt,
		"health": map[string]interface{}{
			"status": health,
		},
		"progress": map[string]interface{}{
			"duration_seconds":          exp.Duration,
			"elapsed_ms":                elapsedMs,
			"phase_elapsed_ms":          phaseElapsedMs,
			"fault_elapsed_ms":          faultElapsedMs,
			"completed_percent_of_plan": progressPercent,
		},
		"timeline": map[string]interface{}{
			"fault_start":  exp.FaultStartedAt,
			"first_impact": firstImpact,
			"recovery":     recovery,
		},
		"faults": map[string]interface{}{
			"scheduled": len(exp.Scenario),
			"applied":   len(history),
			"events":    faultsApplied,
		},
		"state_transitions": transitions,
	}

	// Transparency fields for polling clients: when and what the next scheduled fault step is.
	nextFaultEtaMs := int64(-1)
	var nextFault map[string]interface{}

	isTerminal, _ := status["is_terminal"].(bool)
	if !isTerminal && !exp.FaultStartedAt.IsZero() && len(exp.Scenario) > 0 {
		var nextAt time.Time
		for _, step := range exp.Scenario {
			planned := exp.FaultStartedAt.Add(time.Duration(step.At) * time.Second)
			if !planned.After(now) {
				continue
			}
			if nextAt.IsZero() || planned.Before(nextAt) {
				nextAt = planned
				nextFault = map[string]interface{}{
					"type":       step.Type,
					"at":         planned,
					"at_ms":      o.relativeMillis(exp.FaultStartedAt, planned),
					"in_phase":   exp.Phase,
					"configured": true,
				}
			}
		}
		if !nextAt.IsZero() {
			nextFaultEtaMs = nextAt.Sub(now).Milliseconds()
		}
	}

	status["next_fault_eta_ms"] = nextFaultEtaMs
	status["next_fault"] = nextFault

	impactObserved := len(firstImpact) > 0
	recoveryObserved := len(recovery) > 0
	impactPending := !isTerminal && !impactObserved
	waitingForStep := ""
	if nextFault != nil {
		if t, ok := nextFault["type"].(string); ok {
			waitingForStep = t
		}
	}
	status["timeline_status"] = map[string]interface{}{
		"impact_observed":   impactObserved,
		"recovery_observed": recoveryObserved,
		"impact_pending":    impactPending,
		"waiting_for_step":  waitingForStep,
	}

	return status, nil
}

func androidPrimaryEndpoint(exp *models.Experiment, flat []models.MetricSample) string {
	if len(flat) > 0 && strings.TrimSpace(flat[0].Endpoint) != "" {
		return strings.TrimSpace(flat[0].Endpoint)
	}
	if exp != nil && len(exp.Targets) > 0 {
		return strings.TrimSpace(exp.Targets[0])
	}
	if exp != nil && strings.TrimSpace(exp.Package) != "" {
		return strings.TrimSpace(exp.Package)
	}
	return "android-target"
}

func inferAndroidImpactAndRecovery(samples []models.MetricSample, faultStart time.Time) (time.Time, time.Time) {
	if len(samples) == 0 {
		return time.Time{}, time.Time{}
	}

	impact := time.Time{}
	recovery := time.Time{}
	seenDown := false
	prevState := ""

	for _, sample := range samples {
		if !faultStart.IsZero() && sample.Timestamp.Before(faultStart) {
			continue
		}
		state := classifyAndroidState(sample)
		if !seenDown && (state == "not_running" || state == "crash" || state == "anr" || sample.Crash || sample.ANR) {
			impact = sample.Timestamp
			seenDown = true
		}
		if seenDown && prevState != "running" && state == "running" && !sample.Crash && !sample.ANR {
			recovery = sample.Timestamp
			break
		}
		prevState = state
	}

	return impact, recovery
}

func synthesizeAndroidTimeline(exp *models.Experiment, history []FaultEvent, currentState string, currentSampleAt, now time.Time) (time.Time, time.Time) {
	if exp == nil || len(history) == 0 {
		return time.Time{}, time.Time{}
	}

	impact := time.Time{}
	recovery := time.Time{}

	for _, ev := range history {
		if !exp.FaultStartedAt.IsZero() && ev.Timestamp.Before(exp.FaultStartedAt) {
			continue
		}
		faultType := strings.ToLower(strings.TrimSpace(ev.Type))
		if impact.IsZero() && isAndroidImpactFaultType(faultType) {
			impact = ev.Timestamp
			continue
		}
		if !impact.IsZero() && !ev.Timestamp.Before(impact) && isAndroidRecoveryFaultType(faultType) {
			recovery = ev.Timestamp
			break
		}
	}

	if !impact.IsZero() && recovery.IsZero() && isAndroidHealthyState(currentState) {
		if !currentSampleAt.IsZero() && !currentSampleAt.Before(impact) {
			recovery = currentSampleAt
		} else if !now.IsZero() && !now.Before(impact) {
			recovery = now
		}
	}

	if !impact.IsZero() && !recovery.IsZero() && recovery.Before(impact) {
		recovery = time.Time{}
	}

	return impact, recovery
}

func isAndroidImpactFaultType(t string) bool {
	switch t {
	case "kill_app", "kill_repeated", "network_disable", "network_flaky", "network_latency", "network_loss", "background_app", "clear_data", "revoke_camera", "revoke_storage", "revoke_location":
		return true
	default:
		return false
	}
}

func isAndroidRecoveryFaultType(t string) bool {
	switch t {
	case "network_enable", "foreground_app":
		return true
	default:
		return false
	}
}

func isAndroidHealthyState(state string) bool {
	return state == "running"
}
