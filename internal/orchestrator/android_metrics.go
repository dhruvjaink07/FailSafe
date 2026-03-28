package orchestrator

import (
	"sort"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

type androidTransition struct {
	from string
	to   string
	at   time.Time
}

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
	backgroundCount := 0
	warningCount := 0
	anr := false
	hadIncident := false
	hadNotRunning := false
	hadCrashState := false
	crashReason := ""
	crashThread := ""
	crashClassCounts := map[string]int{
		"ui_bug":        0,
		"network_bug":   0,
		"lifecycle_bug": 0,
		"unknown":       0,
	}
	transitions := make([]androidTransition, 0)
	previousState := ""
	unexpectedRestarts := 0

	for _, s := range flat {
		state := classifyAndroidState(s)

		if s.Crash {
			crashCount++
			hadCrashState = true
			hadIncident = true
			if crashReason == "" && s.CrashReason != "" {
				crashReason = s.CrashReason
				crashThread = s.CrashThread
			}
			className := s.CrashClass
			if className == "" {
				className = "unknown"
			}
			crashClassCounts[className]++
		}
		if s.ANR {
			anr = true
			hadIncident = true
			if crashReason == "" {
				crashReason = "ANR detected from Android traces"
			}
		}
		if s.Warning {
			warningCount++
			hadIncident = true
		}
		if state == "running" {
			runningCount++
		}
		if state == "background" {
			backgroundCount++
		}
		if state == "not_running" {
			hadNotRunning = true
			hadIncident = true
		}
		if state == "crash" {
			hadCrashState = true
			hadIncident = true
		}
		if state != "running" {
			hadIncident = true
		}

		if previousState != "" && previousState != state {
			transitions = append(transitions, androidTransition{from: previousState, to: state, at: s.Timestamp})
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
	if crashRate > 0 || hadCrashState {
		failureType = "crash"
	} else if anr {
		failureType = "anr"
	} else if hadNotRunning {
		failureType = "killed"
	} else if warningCount > 0 {
		failureType = "healthy"
	}

	firstImpactCopy := make(map[string]time.Time)
	o.mu.Lock()
	for ep, ts := range o.firstImpact[id] {
		firstImpactCopy[ep] = ts
	}
	o.mu.Unlock()

	if crashReason == "" && failureType != "healthy" {
		cause := o.probableCause(id, firstImpactCopy)
		switch failureType {
		case "crash":
			if cause != "" {
				crashReason = "App crash observed after fault: " + cause
			} else {
				crashReason = "App crash observed during experiment"
			}
		case "anr":
			if cause != "" {
				crashReason = "ANR observed after fault: " + cause
			} else {
				crashReason = "ANR observed during experiment"
			}
		case "killed":
			if cause != "" {
				crashReason = "Process not running after fault: " + cause
			} else {
				crashReason = "Process became not_running during experiment"
			}
		}
	}

	severity := "low"
	if crashRate > 50 {
		severity = "critical"
	} else if crashRate > 0 || anr {
		severity = "high"
	} else if hadNotRunning {
		severity = "high"
	} else if warningCount > 0 {
		severity = "medium"
	}

	status := "healthy"
	if failureType == "crash" || failureType == "anr" || failureType == "killed" {
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
		steps := exp.IntensityHistory
		if steps == nil {
			steps = []int{}
		}
		resilience["max_stable_intensity"] = exp.MaxStableIntensity
		resilience["breaking_intensity"] = exp.BreakingIntensity
		resilience["intensity_steps"] = steps
	} else {
		resilience["max_stable_intensity"] = 0
		resilience["breaking_intensity"] = 0
		resilience["intensity_steps"] = []int{}
	}

	runningNow := previousState == "running"
	validation := map[string]interface{}{
		"configured": false,
		"passed":     true,
		"reasons":    []string{},
		"expected":   map[string]interface{}{},
	}

	if exp != nil {
		expected := exp.Expected
		reasons := make([]string, 0)
		configured := expected.AppState != "" || expected.Running != nil || expected.NotCrash || expected.NotANR

		if expected.Running != nil && *expected.Running != runningNow {
			reasons = append(reasons, "running state does not match expectation")
		}
		if expected.AppState != "" && expected.AppState != previousState {
			reasons = append(reasons, "app_state does not match expectation")
		}
		if expected.NotCrash && crashCount > 0 {
			reasons = append(reasons, "crash detected but expected not_crash=true")
		}
		if expected.NotANR && anr {
			reasons = append(reasons, "anr detected but expected not_anr=true")
		}

		validation = map[string]interface{}{
			"configured": configured,
			"passed":     len(reasons) == 0,
			"reasons":    reasons,
			"expected": map[string]interface{}{
				"app_state": expected.AppState,
				"running":   expected.Running,
				"not_crash": expected.NotCrash,
				"not_anr":   expected.NotANR,
			},
		}
	}

	replayHints := o.buildAndroidReplayHints(id, faultStart, transitions)

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
			"warning_signals":     warningCount,
			"background_samples":  backgroundCount,
		},
		"recovery": map[string]interface{}{
			"auto_recovered":               autoRecovered,
			"recovery_time_ms":             recoveryTimeMs,
			"manual_intervention_required": manualIntervention,
			"running":                      runningNow,
		},
		"crash_classification": crashClassCounts,
		"validation":           validation,
		"replay_hints":         replayHints,
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

func (o *Orchestrator) buildAndroidReplayHints(id string, start time.Time, transitions []androidTransition) []map[string]interface{} {
	hints := make([]map[string]interface{}, 0)

	for _, ev := range o.getFaultHistory(id) {
		hints = append(hints, map[string]interface{}{
			"step":    "inject_fault",
			"fault":   ev.Type,
			"at_ms":   o.relativeMillis(start, ev.Timestamp),
			"at_time": ev.Timestamp,
		})
	}

	for _, tr := range transitions {
		hints = append(hints, map[string]interface{}{
			"step":    "state_transition",
			"from":    tr.from,
			"to":      tr.to,
			"at_ms":   o.relativeMillis(start, tr.at),
			"at_time": tr.at,
		})
	}

	sort.Slice(hints, func(i, j int) bool {
		left, _ := hints[i]["at_ms"].(int64)
		right, _ := hints[j]["at_ms"].(int64)
		return left < right
	})

	return hints
}
