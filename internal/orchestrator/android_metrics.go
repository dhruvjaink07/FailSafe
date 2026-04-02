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
	primaryEndpoint := ""
	if len(flat) > 0 && flat[0].Endpoint != "" {
		primaryEndpoint = flat[0].Endpoint
	}
	if primaryEndpoint == "" && exp != nil && len(exp.Targets) > 0 {
		primaryEndpoint = exp.Targets[0]
	}
	if primaryEndpoint == "" && exp != nil && exp.Package != "" {
		primaryEndpoint = exp.Package
	}
	if primaryEndpoint == "" {
		primaryEndpoint = o.pkg
	}
	if primaryEndpoint == "" {
		primaryEndpoint = "android-target"
	}

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
	lastRunningTransitionAt := time.Time{}
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
			if state == "running" {
				lastRunningTransitionAt = s.Timestamp
			}
			if previousState == "not_running" && state == "running" {
				unexpectedRestarts++
			}
		}

		previousState = state
	}

	if hadNotRunning || failureTypeFromState(flat) == "killed" {
		// refined below once induced vs intrinsic kill cause is known
	}

	crashRate := 0.0
	uptimePercent := 0.0
	if total > 0 {
		crashRate = float64(crashCount) / float64(total) * 100
		uptimePercent = float64(runningCount) / float64(total) * 100
	}

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
	recoveryCopy := make(map[string]time.Time)
	o.mu.Lock()
	for ep, ts := range o.firstImpact[id] {
		firstImpactCopy[ep] = ts
	}
	for ep, ts := range o.recoveryAt[id] {
		recoveryCopy[ep] = ts
	}
	o.mu.Unlock()

	firstImpactAt, recoveryAt := findImpactAndRecoveryFromTransitions(transitions)
	if firstImpactAt.IsZero() && len(flat) > 0 {
		firstImpactAt, recoveryAt = inferAndroidImpactAndRecovery(flat, exp.FaultStartedAt)
	}
	if !firstImpactAt.IsZero() || !recoveryAt.IsZero() {
		o.mu.Lock()
		if _, ok := o.firstImpact[id]; !ok {
			o.firstImpact[id] = make(map[string]time.Time)
		}
		if _, ok := o.recoveryAt[id]; !ok {
			o.recoveryAt[id] = make(map[string]time.Time)
		}
		if !firstImpactAt.IsZero() {
			o.firstImpact[id][primaryEndpoint] = firstImpactAt
		}
		if !recoveryAt.IsZero() {
			o.recoveryAt[id][primaryEndpoint] = recoveryAt
		}
		o.mu.Unlock()
	}

	if len(firstImpactCopy) == 0 {
		if !firstImpactAt.IsZero() {
			firstImpactCopy[primaryEndpoint] = firstImpactAt
		}
	}
	if firstImpactAt.IsZero() {
		firstImpactAt = firstImpactCopy[primaryEndpoint]
	}

	if currentRecovery, ok := recoveryCopy[primaryEndpoint]; ok && !firstImpactAt.IsZero() && currentRecovery.Before(firstImpactAt) {
		delete(recoveryCopy, primaryEndpoint)
	}

	if !recoveryAt.IsZero() {
		if firstImpactAt.IsZero() || !recoveryAt.Before(firstImpactAt) {
			recoveryCopy[primaryEndpoint] = recoveryAt
		}
	}

	faultHistory := o.getFaultHistory(id)
	scenarioLabel := deriveAndroidScenarioLabel(exp, faultHistory)

	recovered, recoveryAtResolved := hasRecoveredAfterImpact(firstImpactAt, recoveryCopy)
	stableRecovered, stableRecoveryAt := hasStableRecovery(flat, firstImpactAt, 8*time.Second)
	if !recovered && stableRecovered {
		recovered = true
		recoveryAtResolved = stableRecoveryAt
	}
	if !recovered && previousState == "running" && !firstImpactAt.IsZero() {
		recovered = true
		if !lastRunningTransitionAt.IsZero() && !lastRunningTransitionAt.Before(firstImpactAt) {
			recoveryAtResolved = lastRunningTransitionAt
		} else if len(flat) > 0 && !flat[len(flat)-1].Timestamp.Before(firstImpactAt) {
			recoveryAtResolved = flat[len(flat)-1].Timestamp
		}
	}
	recoveryTimeMs := int64(-1)
	if recovered && !firstImpactAt.IsZero() && !recoveryAtResolved.Before(firstImpactAt) {
		recoveryTimeMs = recoveryAtResolved.Sub(firstImpactAt).Milliseconds()
		recoveryCopy[primaryEndpoint] = recoveryAtResolved
	}
	if recoveryTimeMs < 0 {
		recoveryTimeMs = -1
	}

	likelyCause := o.probableCause(id, firstImpactCopy)
	inducedKill := failureType == "killed" && (likelyCause == "kill_app" || likelyCause == "kill_repeated")
	if hadNotRunning {
		if inducedKill {
			crashClassCounts["induced_kill"] = 1
		} else {
			crashClassCounts["lifecycle_bug"]++
		}
	}

	autoRecovered := recovered && !wasRecoveryExternallyTriggered(faultHistory, firstImpactAt, recoveryAtResolved)

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
	} else if failureType == "healthy" {
		if warningCount > 0 {
			severity = "medium"
		}
	} else if !recovered {
		severity = "critical"
	} else if failureType == "killed" || failureType == "crash" || failureType == "anr" {
		severity = "high"
	} else {
		severity = "medium"
	}

	status := "healthy"
	if failureType == "crash" || failureType == "anr" || failureType == "killed" {
		if recovered {
			status = "degraded"
		} else {
			status = "down"
		}
	} else if hadIncident {
		status = "degraded"
	}

	manualIntervention := !autoRecovered && failureType != "healthy"

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
		"passed":     nil,
		"reasons":    []string{},
		"expected":   map[string]interface{}{},
	}

	if exp != nil {
		expected := exp.Expected
		reasons := make([]string, 0)
		configured := expected.AppState != "" || expected.Running != nil || expected.NotCrash || expected.NotANR || expected.ShouldRecover != nil

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
		if expected.ShouldRecover != nil && *expected.ShouldRecover {
			if !recovered {
				reasons = append(reasons, "expected recovery but app did not recover")
			} else if !autoRecovered {
				reasons = append(reasons, "app recovered but required external interaction")
			}
		}
		if expected.NotCrash && failureType == "killed" && !recovered {
			reasons = append(reasons, "process kill detected and app did not recover")
		}

		passed := interface{}(nil)
		if configured {
			passed = len(reasons) == 0
		}

		validation = map[string]interface{}{
			"configured": configured,
			"passed":     passed,
			"reasons":    reasons,
			"expected": map[string]interface{}{
				"app_state":      expected.AppState,
				"running":        expected.Running,
				"not_crash":      expected.NotCrash,
				"not_anr":        expected.NotANR,
				"should_recover": expected.ShouldRecover,
			},
		}
	}

	summaryResult := "UNKNOWN"
	summaryReason := "No expectation configured; results are observational"
	summarySuggestion := "Set expected.should_recover and expected.not_crash to enforce pass/fail assertions"

	if vConfigured, _ := validation["configured"].(bool); vConfigured {
		if vPassed, ok := validation["passed"].(bool); ok {
			if vPassed {
				summaryResult = "PASS"
				summaryReason = "Observed behavior matched configured expectations"
				summarySuggestion = "Increase stress with network_flaky + kill_repeated to probe deeper resilience"
			} else {
				summaryResult = "FAIL"
				summaryReason = failureReason(failureType, recovered, autoRecovered)
				summarySuggestion = failureSuggestion(failureType, autoRecovered)
			}
		}
	} else if failureType != "healthy" {
		summaryResult = "FAIL"
		summaryReason = failureReason(failureType, recovered, autoRecovered)
		summarySuggestion = failureSuggestion(failureType, autoRecovered)
	}

	if likelyCause != "" && summaryResult == "FAIL" {
		summaryReason = summaryReason + " (likely cause: " + likelyCause + ")"
	}

	if recovered && recoveryTimeMs >= 10000 {
		if severity == "medium" {
			severity = "high"
		}
		if recoveryTimeMs >= 30000 {
			severity = "critical"
		}
	}

	replayHints := o.buildAndroidReplayHints(id, faultStart, transitions)

	return map[string]interface{}{
		"target_type": "android",
		"scenario":    scenarioLabel,
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
			"recovered":                    recovered,
			"stable_recovered":             stableRecovered,
			"recovery_time_ms":             recoveryTimeMs,
			"manual_intervention_required": manualIntervention,
			"running":                      runningNow,
		},
		"crash_classification": crashClassCounts,
		"validation":           validation,
		"summary": map[string]interface{}{
			"result":     summaryResult,
			"reason":     summaryReason,
			"suggestion": summarySuggestion,
		},
		"replay_hints":      replayHints,
		"state_transitions": stateTransitions,
		"timeline": map[string]interface{}{
			"fault_start":  faultStart,
			"first_impact": firstImpactCopy,
			"recovery":     recoveryCopy,
		},
		"resilience_threshold": resilience,
		"blast_radius_percent": 0,
		"cascade_depth":        0,
	}
}

func hasStableRecovery(samples []models.MetricSample, since time.Time, window time.Duration) (bool, time.Time) {
	if window <= 0 {
		window = 8 * time.Second
	}

	stableStart := time.Time{}
	seenFailure := since.IsZero()
	for _, s := range samples {
		if !since.IsZero() && s.Timestamp.Before(since) {
			continue
		}
		state := classifyAndroidState(s)
		if !seenFailure {
			if state == "not_running" || state == "crash" || state == "anr" || state == "degraded" {
				seenFailure = true
			}
			continue
		}

		if state == "running" && !s.Crash && !s.ANR {
			if stableStart.IsZero() {
				stableStart = s.Timestamp
			}
			if s.Timestamp.Sub(stableStart) >= window {
				return true, s.Timestamp
			}
			continue
		}
		stableStart = time.Time{}
	}

	return false, time.Time{}
}

func deriveAndroidScenarioLabel(exp *models.Experiment, history []FaultEvent) string {
	faults := make(map[string]bool)
	for _, ev := range history {
		faults[ev.Type] = true
	}

	if faults["kill_app"] && faults["foreground_app"] {
		return "process_kill_recovery"
	}
	if (faults["network_disable"] || faults["network_flaky"]) && faults["network_enable"] {
		return "network_interruption_recovery"
	}
	if faults["background_app"] && faults["foreground_app"] {
		return "lifecycle_background_foreground"
	}
	if faults["revoke_camera"] || faults["revoke_storage"] || faults["revoke_location"] {
		return "permission_resilience"
	}

	if exp != nil && len(exp.Scenario) > 0 {
		return "custom_scenario"
	}

	return "single_fault"
}

func hasRecoveredAfterImpact(firstImpactAt time.Time, recovery map[string]time.Time) (bool, time.Time) {
	best := time.Time{}
	for _, ts := range recovery {
		if ts.IsZero() {
			continue
		}
		if !firstImpactAt.IsZero() && ts.Before(firstImpactAt) {
			continue
		}
		if best.IsZero() || ts.Before(best) {
			best = ts
		}
	}

	if best.IsZero() {
		return false, time.Time{}
	}
	return true, best
}

func findImpactAndRecoveryFromTransitions(transitions []androidTransition) (time.Time, time.Time) {
	impact := time.Time{}
	recovery := time.Time{}

	for _, tr := range transitions {
		if impact.IsZero() && (tr.to == "not_running" || tr.to == "crash" || tr.to == "anr" || tr.to == "degraded") {
			impact = tr.at
			continue
		}

		if !impact.IsZero() && tr.from != "running" && tr.to == "running" && !tr.at.Before(impact) {
			recovery = tr.at
			break
		}
	}

	return impact, recovery
}

func wasRecoveryExternallyTriggered(history []FaultEvent, impactAt, recoveryAt time.Time) bool {
	if impactAt.IsZero() || recoveryAt.IsZero() {
		return false
	}

	for _, ev := range history {
		if ev.Type != "foreground_app" {
			continue
		}
		if ev.Timestamp.Before(impactAt) {
			continue
		}
		if ev.Timestamp.After(recoveryAt.Add(2 * time.Second)) {
			continue
		}
		return true
	}

	return false
}

func failureReason(failureType string, recovered bool, autoRecovered bool) string {
	switch failureType {
	case "killed":
		if !recovered {
			return "App did not recover after process kill"
		}
		if !autoRecovered {
			return "App recovered only after external trigger"
		}
		return "App process was killed and later recovered"
	case "crash":
		if !recovered {
			return "App crashed and did not recover stably"
		}
		if !autoRecovered {
			return "App recovered only after external trigger"
		}
		return "App crashed but recovered"
	case "anr":
		if !recovered {
			return "App became unresponsive (ANR) and did not recover"
		}
		if !autoRecovered {
			return "App recovered from ANR only after external trigger"
		}
		return "App hit ANR but recovered"
	default:
		return "No hard failure detected"
	}
}

func failureSuggestion(failureType string, autoRecovered bool) string {
	switch failureType {
	case "killed":
		if !autoRecovered {
			return "Handle process death and restore app state on launch"
		}
		return "Improve warm-start rehydration to reduce restart impact"
	case "crash":
		return "Capture crash stack traces and harden lifecycle/state restoration paths"
	case "anr":
		return "Move heavy work off main thread and add timeout guards for blocking operations"
	default:
		if !autoRecovered {
			return "Add retry logic and defensive recovery hooks for transient failures"
		}
		return "Increase stress scenarios to validate deeper resilience behavior"
	}
}

func failureTypeFromState(samples []models.MetricSample) string {
	for _, s := range samples {
		state := classifyAndroidState(s)
		if state == "not_running" {
			return "killed"
		}
	}
	return "healthy"
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
