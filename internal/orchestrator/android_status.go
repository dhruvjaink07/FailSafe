package orchestrator

import (
	"errors"
	"sort"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) GetAndroidStatus(id string) (map[string]interface{}, error) {
	o.mu.Lock()
	exp, ok := o.experiments[id]
	if !ok {
		o.mu.Unlock()
		return nil, errors.New("experiment not found")
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
	health := "healthy"
	if currentState == "not_running" || currentState == "crash" || currentState == "anr" {
		health = "down"
	} else if hadImpact || currentState == "background" || currentState == "degraded" {
		health = "degraded"
	}

	history := o.getFaultHistory(id)
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
		faultsApplied = append(faultsApplied, map[string]interface{}{
			"type":     ev.Type,
			"at":       ev.Timestamp,
			"at_ms":    o.relativeMillis(exp.FaultStartedAt, ev.Timestamp),
			"in_phase": exp.Phase,
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

	status := map[string]interface{}{
		"id":                exp.ID,
		"target_type":       exp.TargetType,
		"observation_type":  exp.ObservationType,
		"state":             exp.State,
		"phase":             exp.Phase,
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

	return status, nil
}
