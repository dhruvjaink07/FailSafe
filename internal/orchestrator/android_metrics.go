package orchestrator

import (
	"sort"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) getAndroidMetrics(id string) map[string]interface{} {
	o.mu.Lock()
	exp := o.experiments[id]
	samples := o.metrics[id]
	o.mu.Unlock()

	flat := make([]models.MetricSample, 0)
	for _, list := range samples {
		flat = append(flat, list...)
	}

	sort.Slice(flat, func(i, j int) bool {
		return flat[i].Timestamp.Before(flat[j].Timestamp)
	})

	total := len(flat)
	crashCount := 0
	runningCount := 0
	anr := false
	hadIncident := false
	crashReason := ""
	crashThread := ""

	type transition struct {
		from string
		to   string
		at   time.Time
	}

	transitions := make([]transition, 0)
	previousState := ""
	unexpectedRestarts := 0

	for _, s := range flat {
		state := classifyAndroidState(s)

		if s.Crash {
			crashCount++
			hadIncident = true
			if crashReason == "" && s.CrashReason != "" {
				crashReason = s.CrashReason
				crashThread = s.CrashThread
			}
		}
		if s.ANR {
			anr = true
			hadIncident = true
		}
		if state == "running" {
			runningCount++
		}
		if state != "running" {
			hadIncident = true
		}

		if previousState != "" && previousState != state {
			transitions = append(transitions, transition{from: previousState, to: state, at: s.Timestamp})
			if previousState == "not_running" && state == "running" {
				unexpectedRestarts++
			}
		}

		previousState = state
	}

	crashRate := 0.0
	uptimePercent := 0.0
	if total > 0 {
		crashRate = float64(crashCount) / float64(total) * 100
		uptimePercent = float64(runningCount) / float64(total) * 100
	}

	recoveryTimeMs := int64(-1)
	o.mu.Lock()
	for ep, start := range o.firstImpact[id] {
		if rec, ok := o.recoveryAt[id][ep]; ok {
			recoveryTimeMs = rec.Sub(start).Milliseconds()
			break
		}
	}
	o.mu.Unlock()

	failureType := "healthy"
	if crashRate > 0 {
		failureType = "crash"
	} else if anr {
		failureType = "anr"
	} else if previousState == "not_running" {
		failureType = "killed"
	}

	severity := "low"
	if crashRate > 50 {
		severity = "critical"
	} else if crashRate > 0 || anr {
		severity = "high"
	} else if previousState == "not_running" {
		severity = "medium"
	}

	status := "healthy"
	if previousState == "not_running" || failureType == "crash" || failureType == "anr" {
		status = "down"
	} else if hadIncident {
		status = "degraded"
	}

	autoRecovered := recoveryTimeMs >= 0
	manualIntervention := status == "down" && !autoRecovered

	stateTransitions := make([]map[string]interface{}, 0, len(transitions))
	for _, tr := range transitions {
		stateTransitions = append(stateTransitions, map[string]interface{}{
			"from": tr.from,
			"to":   tr.to,
			"at":   tr.at,
		})
	}

	faultStart := time.Time{}
	if exp != nil {
		faultStart = exp.FaultStartedAt
	}

	resilience := map[string]interface{}{}
	if exp != nil {
		resilience["max_stable_intensity"] = exp.MaxStableIntensity
		resilience["breaking_intensity"] = exp.BreakingIntensity
		resilience["intensity_steps"] = exp.IntensityHistory
	} else {
		resilience["max_stable_intensity"] = 0
		resilience["breaking_intensity"] = 0
		resilience["intensity_steps"] = []int{}
	}

	runningNow := previousState == "running"

	return map[string]interface{}{
		"target_type": "android",
		"health": map[string]interface{}{
			"status":       status,
			"failure_type": failureType,
			"severity":     severity,
			"crash_reason": crashReason,
			"thread":       crashThread,
		},
		"stability": map[string]interface{}{
			"crash_rate_percent":  crashRate,
			"anr_detected":        anr,
			"uptime_percent":      uptimePercent,
			"unexpected_restarts": unexpectedRestarts,
		},
		"recovery": map[string]interface{}{
			"auto_recovered":               autoRecovered,
			"recovery_time_ms":             recoveryTimeMs,
			"manual_intervention_required": manualIntervention,
			"running":                      runningNow,
		},
		"state_transitions":    stateTransitions,
		"timeline":             o.buildTimelinePayload(id, faultStart),
		"resilience_threshold": resilience,
		"blast_radius_percent": 0,
		"cascade_depth":        0,
	}
}

func classifyAndroidState(s models.MetricSample) string {
	if s.Crash {
		return "crash"
	}
	if s.ANR {
		return "anr"
	}
	if s.AppState == "not_running" {
		return "not_running"
	}
	if s.AppState != "" {
		return s.AppState
	}
	return "running"
}

func (o *Orchestrator) buildTimelinePayload(id string, faultStart time.Time) map[string]interface{} {
	o.mu.Lock()
	firstImpact := make(map[string]time.Time)
	for ep, t := range o.firstImpact[id] {
		firstImpact[ep] = t
	}

	recovery := make(map[string]time.Time)
	for ep, t := range o.recoveryAt[id] {
		recovery[ep] = t
	}
	o.mu.Unlock()

	return map[string]interface{}{
		"fault_start":  faultStart,
		"first_impact": firstImpact,
		"recovery":     recovery,
	}
}
