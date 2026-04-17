package orchestrator

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
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
	// mark experiments that intentionally shut down services (kill)
	isExpectedDown := false
	if strings.Contains(strings.ToLower(faultType), "kill") {
		isExpectedDown = true
	}

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

		Adaptive:            adaptive,
		StepIntensity:       stepIntensity,
		MaxIntensity:        maxIntensity,
		DependencyGraph:     deps,
		TargetEndpointMap:   targetMap,
		TimelineHistory:     make(map[int]models.IntensityTimeline),
		ExpectedServiceDown: isExpectedDown,
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
	o.stopFrontendRunner(id)

	if o.db != nil {
		_ = o.flushMetricsBatch(id)
		_ = o.db.UpdateExperimentResults(exp)

		if metrics, err := o.GetMetrics(id); err == nil {
			if data, ok := metrics.(map[string]interface{}); ok {
				_ = o.db.InsertPlatformStatusMetrics(exp, data)
				if exp.TargetType == "android" || exp.ObservationType == "android" {
					_ = o.db.InsertAndroidExperimentReport(id, data)
					_ = o.db.InsertAndroidExperimentSummary(id, data)
				}
			}
		}
	}
}

func (o *Orchestrator) GetExperiment(id string) (map[string]interface{}, error) {
	o.mu.Lock()
	exp, ok := o.experiments[id]
	o.mu.Unlock()

	if !ok {
		if o.db == nil {
			return nil, errors.New("experiment not found")
		}

		targetType, err := o.db.GetExperimentTargetType(id)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(targetType) == "" {
			return nil, errors.New("experiment not found")
		}

		exp, err = o.db.GetExperimentSnapshot(id, targetType)
		if err != nil {
			return nil, err
		}
		if exp == nil {
			return nil, errors.New("experiment not found")
		}
	}

	// Prepare fault_injection_history for status output (v1 and v2)
	var faultInjectionHistory []map[string]interface{}
	var faultInjectionHistoryV2 []map[string]interface{}
	var allRecoveryDurations []int64
	var allDowntimeDurations []int64
	var allFailures int
	var anomalyEndpoints []string
	for intensity, entry := range exp.TimelineHistory {
		hist := map[string]interface{}{
			"intensity":        intensity,
			"fault_started_at": entry.FaultStartedAt,
			"first_impact":     entry.FirstImpact,
			"recovery_at":      entry.RecoveryAt,
		}
		faultInjectionHistory = append(faultInjectionHistory, hist)

		// --- v2: durations, endpoint impact, cause, cascade, anomalies ---
		impacts := map[string]map[string]interface{}{}
		for ep, impactTime := range entry.FirstImpact {
			recTime, hasRec := entry.RecoveryAt[ep]
			var duration int64 = -1
			if hasRec {
				duration = recTime.Sub(impactTime).Milliseconds()
				// If experiment intentionally shuts down services, and this
				// endpoint belongs to a targeted endpoint list, do not count
				// its downtime in aggregated recovery/downtime stats.
				ignore := false
				if exp.ExpectedServiceDown && len(exp.TargetEndpointMap) > 0 {
					for _, eps := range exp.TargetEndpointMap {
						for _, e := range eps {
							if e == ep {
								ignore = true
								break
							}
						}
						if ignore {
							break
						}
					}
				}
				if !ignore {
					allRecoveryDurations = append(allRecoveryDurations, duration)
				}
			}
			// Down duration: from first impact to recovery
			impacts[ep] = map[string]interface{}{
				"first_impact": impactTime,
				"recovery_at":  recTime,
				"duration_ms":  duration,
			}
			if duration > 0 {
				// append to total downtime only if not an expected shutdown
				skip := false
				if exp.ExpectedServiceDown && len(exp.TargetEndpointMap) > 0 {
					for _, eps := range exp.TargetEndpointMap {
						for _, e := range eps {
							if e == ep {
								skip = true
								break
							}
						}
						if skip {
							break
						}
					}
				}
				if !skip {
					allDowntimeDurations = append(allDowntimeDurations, duration)
				}
			}
		}
		// Fault cause and cascade path
		cause := o.probableCause(id, entry.FirstImpact)
		// Cascade path: endpoints affected in order of first impact
		cascade := []string{}
		type epImpact struct {
			ep string
			t  time.Time
		}
		var impactsList []epImpact
		for ep, t := range entry.FirstImpact {
			impactsList = append(impactsList, epImpact{ep, t})
		}
		sort.Slice(impactsList, func(i, j int) bool { return impactsList[i].t.Before(impactsList[j].t) })
		for _, v := range impactsList {
			cascade = append(cascade, v.ep)
		}

		// Anomaly detection: endpoints with recovery > 2x median
		var anomalyList []string
		var durations []int64
		for epKey, v := range impacts {
			// skip durations for expected shutdown endpoints
			if exp.ExpectedServiceDown && len(exp.TargetEndpointMap) > 0 {
				skip := false
				for _, eps := range exp.TargetEndpointMap {
					for _, e := range eps {
						if e == epKey {
							skip = true
							break
						}
					}
					if skip {
						break
					}
				}
				if skip {
					continue
				}
			}
			if d, ok := v["duration_ms"].(int64); ok && d > 0 {
				durations = append(durations, d)
			}
		}
		var median int64
		if len(durations) > 0 {
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			median = durations[len(durations)/2]
			for ep, v := range impacts {
				// skip expected shutdown endpoints when classifying anomalies
				if exp.ExpectedServiceDown && len(exp.TargetEndpointMap) > 0 {
					skip := false
					for _, eps := range exp.TargetEndpointMap {
						for _, e := range eps {
							if e == ep {
								skip = true
								break
							}
						}
						if skip {
							break
						}
					}
					if skip {
						continue
					}
				}
				if d, ok := v["duration_ms"].(int64); ok && d > 2*median && median > 0 {
					anomalyList = append(anomalyList, ep)
				}
			}
		}
		if len(anomalyList) > 0 {
			anomalyEndpoints = append(anomalyEndpoints, anomalyList...)
		}
		faultInjectionHistoryV2 = append(faultInjectionHistoryV2, map[string]interface{}{
			"intensity":        intensity,
			"fault_started_at": entry.FaultStartedAt,
			"impacts":          impacts,
			"fault_cause":      cause,
			"cascade_path":     cascade,
			"anomalies":        anomalyList,
		})
	}

	// Optionally, you can return a struct with both exp and faultInjectionHistory if your API handler supports it
	// For now, just return exp (API handler should be updated to include faultInjectionHistory in the response)
	// Attach new fields to exp via a map for API handler
	expMap := map[string]interface{}{}
	// Copy all exported fields from exp (for backward compatibility)
	// This is a shallow copy; for deep copy, use a helper if needed
	// Use JSON marshal/unmarshal for deep copy if required
	// For now, just add new fields
	expMap["experiment"] = exp
	expMap["fault_injection_history"] = faultInjectionHistory
	expMap["fault_injection_history_v2"] = faultInjectionHistoryV2
	// Aggregate stats
	var meanRecovery, medianRecovery int64
	if len(allRecoveryDurations) > 0 {
		sum := int64(0)
		for _, d := range allRecoveryDurations {
			sum += d
		}
		meanRecovery = sum / int64(len(allRecoveryDurations))
		sorted := append([]int64(nil), allRecoveryDurations...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		medianRecovery = sorted[len(sorted)/2]
	}
	expMap["aggregate_stats"] = map[string]interface{}{
		"total_downtime_ms":  allDowntimeDurations,
		"mean_recovery_ms":   meanRecovery,
		"median_recovery_ms": medianRecovery,
		"total_failures":     allFailures,
		"anomaly_endpoints":  anomalyEndpoints,
	}
	return expMap, nil
}

func (o *Orchestrator) StopExperiment(id string) error {
	o.mu.Lock()
	exp, ok := o.experiments[id]
	if !ok {
		o.mu.Unlock()
		if o.db == nil {
			return errors.New("experiment not found")
		}

		targetType, err := o.db.GetExperimentTargetType(id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(targetType) == "" {
			return errors.New("experiment not found")
		}

		exp, err = o.db.GetExperimentSnapshot(id, targetType)
		if err != nil {
			return err
		}
		if exp == nil {
			return errors.New("experiment not found")
		}

		exp.State = models.StateFailed
		exp.Phase = models.PhaseCompleted
		exp.UpdatedAt = time.Now()

		_ = o.db.UpdateExperimentResults(exp)
		_ = o.db.InsertPlatformExperiment(exp)

		if payload, err := o.db.GetPlatformStatusPayload(targetType, id); err == nil && payload != nil {
			payload["state"] = string(exp.State)
			payload["phase"] = string(exp.Phase)

			if expMap, ok := payload["experiment"].(map[string]interface{}); ok {
				expMap["state"] = string(exp.State)
				expMap["phase"] = string(exp.Phase)
				expMap["updated_at"] = exp.UpdatedAt
				payload["experiment"] = expMap
			}

			_ = o.db.InsertPlatformStatusMetrics(exp, payload)
		}

		return nil
	}

	monitor := o.monitors[id]
	exp.State = models.StateFailed
	exp.Phase = models.PhaseCompleted
	exp.UpdatedAt = time.Now()
	o.mu.Unlock()

	if monitor != nil {
		monitor.Stop()
	}
	o.stopFrontendRunner(id)

	if o.db != nil {
		_ = o.flushMetricsBatch(id)
		_ = o.db.UpdateExperimentResults(exp)
		if metrics, err := o.GetMetrics(id); err == nil {
			if data, ok := metrics.(map[string]interface{}); ok {
				_ = o.db.InsertPlatformStatusMetrics(exp, data)
			}
		}
	}

	return nil
}

func (o *Orchestrator) GetExperimentTargetType(id string) (string, error) {
	o.mu.Lock()
	exp, ok := o.experiments[id]
	o.mu.Unlock()

	if !ok {
		if o.db == nil {
			return "", errors.New("experiment not found")
		}

		targetType, err := o.db.GetExperimentTargetType(id)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(targetType) == "" {
			return "", errors.New("experiment not found")
		}
		return targetType, nil
	}

	targetType := strings.ToLower(strings.TrimSpace(exp.TargetType))
	if targetType == "web" {
		targetType = string(models.TargetFrontend)
	}

	return targetType, nil
}

func (o *Orchestrator) AddFrontendMetrics(data []models.FrontendMetrics) {
	monitorByExperiment := make(map[string]*monitoring.WebMonitor)

	o.mu.Lock()
	for _, metric := range data {
		o.frontendMetrics[metric.ExperimentID] = append(o.frontendMetrics[metric.ExperimentID], metric)
		if mon, ok := o.monitors[metric.ExperimentID].(*monitoring.WebMonitor); ok {
			monitorByExperiment[metric.ExperimentID] = mon
		}
	}
	o.mu.Unlock()

	for _, metric := range data {
		if mon, ok := monitorByExperiment[metric.ExperimentID]; ok {
			mon.RecordIngest(metric)
		}
	}

	if o.db != nil {
		_ = o.db.InsertFrontendMetricsBatch(data)
	}
}

func (o *Orchestrator) GetFrontendMetrics(id string) (interface{}, error) {
	targetType, err := o.GetExperimentTargetType(id)
	if err != nil {
		return nil, err
	}
	if targetType != string(models.TargetFrontend) {
		return nil, errors.New("experiment is not frontend; use backend/android metrics endpoint")
	}

	o.mu.Lock()
	frontend := append([]models.FrontendMetrics(nil), o.frontendMetrics[id]...)
	o.mu.Unlock()

	if len(frontend) == 0 && o.db != nil {
		dbMetrics, err := o.db.GetFrontendMetricsRaw(id)
		if err == nil && len(dbMetrics) > 0 {
			frontend = dbMetrics
		}

		if len(frontend) == 0 {
			if payload, err := o.db.GetPlatformStatusPayload(targetType, id); err == nil && payload != nil {
				return payload, nil
			}
		}
	}

	frontendScoreMap := computeFrontendScore(frontend)

	var frontendScorePtr *float64
	if len(frontend) > 0 {
		if s, ok := frontendScoreMap["score"].(float64); ok {
			frontendScorePtr = &s
		}
	}

	return map[string]interface{}{
		"frontend":       frontend,
		"frontend_score": frontendScoreMap,
		"failsafe_index": computeFailSafeIndex(nil, frontendScorePtr),
	}, nil
}

func computeFrontendScore(metrics []models.FrontendMetrics) map[string]interface{} {
	if len(metrics) == 0 {
		return map[string]interface{}{
			"score":  100,
			"status": "no_data",
		}
	}

	var totalLCP, totalCLS, totalINP float64

	count := float64(len(metrics))
	for _, m := range metrics {
		totalLCP += m.Metrics.LCP
		totalCLS += m.Metrics.CLS
		totalINP += m.Metrics.INP
	}

	avgLCP := totalLCP / count
	avgCLS := totalCLS / count
	avgINP := totalINP / count

	// --- Normalize (basic thresholds) ---
	lcpScore := clamp(100-(avgLCP/40), 0, 100)
	clsScore := clamp(100-(avgCLS*200), 0, 100)
	inpScore := clamp(100-(avgINP/5), 0, 100)

	// --- Weighted score ---
	finalScore := (lcpScore*0.4 + clsScore*0.3 + inpScore*0.3)

	status := "stable"
	if finalScore < 40 {
		status = "critical"
	} else if finalScore < 70 {
		status = "degraded"
	}

	return map[string]interface{}{
		"score":  finalScore,
		"status": status,
		"breakdown": map[string]interface{}{
			"lcp": avgLCP,
			"cls": avgCLS,
			"inp": avgINP,
		},
	}
}

func extractBackendScore(result map[string]interface{}) float64 {

	endpoints, ok := result["endpoints"].(map[string]interface{})
	if !ok || len(endpoints) == 0 {
		return 100
	}

	var total float64
	var count float64

	for _, v := range endpoints {
		ep, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		if score, ok := ep["stability_score"].(float64); ok {
			total += score
			count++
		}
	}

	if count == 0 {
		return 100
	}

	return total / count
}

func computeFailSafeIndex(
	backendScore *float64,
	frontendScore *float64,
) map[string]interface{} {

	// ---- CASE 1: BOTH PRESENT ----
	if backendScore != nil && frontendScore != nil {

		wBackend := 0.6
		wFrontend := 0.4

		final := (*backendScore)*wBackend + (*frontendScore)*wFrontend

		return map[string]interface{}{
			"score": final,
			"mode":  "fullstack",
			"weights": map[string]float64{
				"backend":  wBackend,
				"frontend": wFrontend,
			},
			"status": classify(final),
		}
	}

	// ---- CASE 2: FRONTEND ONLY ----
	if frontendScore != nil {
		return map[string]interface{}{
			"score":  *frontendScore,
			"mode":   "frontend_only",
			"status": classify(*frontendScore),
		}
	}

	// ---- CASE 3: BACKEND ONLY ----
	if backendScore != nil {
		return map[string]interface{}{
			"score":  *backendScore,
			"mode":   "backend_only",
			"status": classify(*backendScore),
		}
	}

	// ---- CASE 4: NO DATA ----
	return map[string]interface{}{
		"score":  100,
		"mode":   "no_data",
		"status": "unknown",
	}
}
