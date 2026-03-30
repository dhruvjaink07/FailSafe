package orchestrator

import (
	"errors"
	"sort"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) GetMetrics(id string) (interface{}, error) {
	o.mu.Lock()
	exp, exists := o.experiments[id]
	if !exists {
		o.mu.Unlock()
		return nil, errors.New("experiment not found")
	}
	o.mu.Unlock()

	if exp.TargetType == "android" || exp.ObservationType == "android" {
		return o.GetAndroidMetrics(id)
	}
	return o.GetBackendMetrics(id)
}

func (o *Orchestrator) GetBackendMetrics(id string) (interface{}, error) {
	o.mu.Lock()
	exp, exists := o.experiments[id]
	if !exists {
		o.mu.Unlock()
		return nil, errors.New("experiment not found")
	}
	if exp.TargetType == "android" || exp.ObservationType == "android" {
		o.mu.Unlock()
		return nil, errors.New("experiment is android; use android metrics endpoint")
	}

	endpointMap, exists := o.metrics[id]
	if !exists {
		o.mu.Unlock()
		return nil, errors.New("no metrics found")
	}
	o.mu.Unlock()

	return o.buildBackendMetrics(id, exp, endpointMap), nil
}

func (o *Orchestrator) GetAndroidMetrics(id string) (interface{}, error) {
	o.mu.Lock()
	exp, exists := o.experiments[id]
	if !exists {
		o.mu.Unlock()
		return nil, errors.New("experiment not found")
	}
	if exp.TargetType != "android" && exp.ObservationType != "android" {
		o.mu.Unlock()
		return nil, errors.New("experiment is backend; use backend metrics endpoint")
	}
	o.mu.Unlock()

	return o.getAndroidMetrics(id), nil
}

func (o *Orchestrator) buildBackendMetrics(id string, exp *models.Experiment, endpointMap map[string][]models.MetricSample) interface{} {

	result := make(map[string]interface{})
	endpointResults := make(map[string]interface{})

	totalEndpoints := 0
	globalRequests := 0

	degradedMap := make(map[string]bool)

	for endpoint, samples := range endpointMap {
		if len(samples) == 0 {
			continue
		}

		totalEndpoints++
		totalRequests := len(samples)
		globalRequests += totalRequests

		var latencies []int64
		var totalCPU float64
		var maxCPU float64
		var totalMem float64
		var maxMem float64
		var errorCount int
		var maxFailureStreak int
		var currentStreak int

		for _, s := range samples {
			latencies = append(latencies, s.LatencyMs)

			if s.Status >= 400 || s.Status == 0 {
				errorCount++
				currentStreak++
				if currentStreak > maxFailureStreak {
					maxFailureStreak = currentStreak
				}
			} else {
				currentStreak = 0
			}

			totalCPU += s.ContainerCPU
			totalMem += s.ContainerMemoryMB

			if s.ContainerCPU > maxCPU {
				maxCPU = s.ContainerCPU
			}
			if s.ContainerMemoryMB > maxMem {
				maxMem = s.ContainerMemoryMB
			}
		}

		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		avgLatency := int64(meanInt64(latencies))
		p50 := percentile(latencies, 50)
		p95 := percentile(latencies, 95)
		p99 := percentile(latencies, 99)
		std := stddev(latencies, avgLatency)
		jit := jitter(latencies)

		errorRate := safeDiv(float64(errorCount)*100, float64(totalRequests))

		latencyRatio := 1.0
		if exp.Baseline.P95 > 0 {
			latencyRatio = float64(p95) / float64(exp.Baseline.P95)
		}

		errorDelta := errorRate - exp.Baseline.ErrorRate

		stability := 100 - (errorRate + (latencyRatio-1)*50)
		if stability < 0 {
			stability = 0
		}
		if stability > 100 {
			stability = 100
		}

		// Mark as degraded if error rate is above 0% or latency ratio is high
		degraded := errorRate > 0 || latencyRatio > 1.5
		if degraded {
			degradedMap[endpoint] = true
		}

		impactOrder := 0
		selectedIntensity := exp.BreakingIntensity
		if selectedIntensity == 0 {
			selectedIntensity = exp.MaxStableIntensity
		}

		if timelineData, ok := exp.TimelineHistory[selectedIntensity]; ok {
			if t, impactExists := timelineData.FirstImpact[endpoint]; impactExists {
				rank := 1
				for ep, other := range timelineData.FirstImpact {
					if ep != endpoint && other.Before(t) {
						rank++
					}
				}
				impactOrder = rank
			}
		}

		endpointResults[endpoint] = map[string]interface{}{
			"requests_total": totalRequests,
			"latency": map[string]interface{}{
				"avg_ms":    avgLatency,
				"p50_ms":    p50,
				"p95_ms":    p95,
				"p99_ms":    p99,
				"stddev_ms": std,
				"jitter_ms": jit,
			},
			"errors": map[string]interface{}{
				"total":              errorCount,
				"rate_percent":       errorRate,
				"max_failure_streak": maxFailureStreak,
			},
			"derived": map[string]interface{}{
				"latency_ratio": latencyRatio,
				"error_delta":   errorDelta,
			},
			"stability_score": stability,
			"impact_order":    impactOrder,
			"container": map[string]interface{}{
				"avg_cpu_percent": safeDiv(totalCPU, float64(totalRequests)),
				"max_cpu_percent": maxCPU,
				"avg_memory_mb":   safeDiv(totalMem, float64(totalRequests)),
				"max_memory_mb":   maxMem,
			},
			"degraded": degraded,
		}
	}

	blast, depth := o.computeGraphImpactScore(exp, degradedMap)

	severity := "isolated"
	if depth >= 3 {
		severity = "systemic"
	} else if depth == 2 {
		severity = "propagated"
	} else if blast > 0 {
		severity = "partial"
	}

	result["endpoints"] = endpointResults
	result["blast_radius_percent"] = blast
	result["cascade_depth"] = depth
	result["system_severity"] = severity
	result["total_requests"] = globalRequests
	result["total_endpoints"] = totalEndpoints

	result["resilience_threshold"] = map[string]interface{}{
		"max_stable_intensity": exp.MaxStableIntensity,
		"breaking_intensity":   exp.BreakingIntensity,
		"intensity_steps":      exp.IntensityHistory,
	}
	result["timeline"] = o.buildTimelinePayload(id, exp.FaultStartedAt)

	return result
}

func (o *Orchestrator) computeBaseline(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	exp := o.experiments[id]
	endpoints := o.metrics[id]

	var allSamples []models.MetricSample

	for _, samples := range endpoints {
		allSamples = append(allSamples, samples...)
	}

	if len(allSamples) == 0 {
		return
	}

	var latencies []int64
	var errorCount int

	for _, s := range allSamples {
		latencies = append(latencies, s.LatencyMs)
		if s.Status >= 400 || s.Status == 0 {
			errorCount++
		}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	avgLatency := meanInt64(latencies)
	p95 := percentile(latencies, 95)
	errorRate := float64(errorCount) / float64(len(allSamples)) * 100

	exp.Baseline = models.BaselineMetrics{
		AvgLatency: avgLatency,
		P95:        p95,
		ErrorRate:  errorRate,
	}
}

func (o *Orchestrator) isExperimentDegraded(id string, since time.Time) bool {
	o.mu.Lock()
	exp := o.experiments[id]
	endpoints := o.metrics[id]
	o.mu.Unlock()

	for _, samples := range endpoints {
		var windowed []models.MetricSample

		for _, s := range samples {
			if s.Timestamp.After(since) && s.Intensity == exp.CurrentIntensity {
				windowed = append(windowed, s)
			}
		}

		if len(windowed) < 5 {
			continue
		}

		if exp.ObservationType == "android" {
			for _, s := range windowed {
				if s.Crash || s.ANR || s.AppState == "not_running" {
					return true
				}
			}
			continue
		}

		var latencies []int64
		var errorCount int

		for _, s := range windowed {
			latencies = append(latencies, s.LatencyMs)
			if s.Status >= 400 || s.Status == 0 {
				errorCount++
			}
		}

		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		if exp.Baseline.P95 == 0 {
			continue
		}

		p95 := percentile(latencies, 95)
		errorRate := float64(errorCount) / float64(len(windowed)) * 100

		latencyRatio := float64(p95) / float64(exp.Baseline.P95)
		errorDelta := errorRate - exp.Baseline.ErrorRate

		if latencyRatio > 1.5 || errorDelta > 5 {
			return true
		}
	}

	return false
}

func (o *Orchestrator) getDegradedEndpoints(id string, since time.Time) map[string]bool {
	result := make(map[string]bool)

	o.mu.Lock()
	exp := o.experiments[id]
	endpoints := o.metrics[id]
	o.mu.Unlock()

	for ep, samples := range endpoints {
		var windowed []models.MetricSample

		for _, s := range samples {
			if s.Timestamp.After(since) && s.Intensity == exp.CurrentIntensity {
				windowed = append(windowed, s)
			}
		}

		if len(windowed) < 5 {
			continue
		}

		if exp.ObservationType == "android" {
			for _, s := range windowed {
				if s.Crash || s.ANR || s.AppState == "not_running" {
					result[ep] = true
					break
				}
			}
			continue
		}

		var latencies []int64
		var errorCount int

		for _, s := range windowed {
			latencies = append(latencies, s.LatencyMs)
			if s.Status >= 400 || s.Status == 0 {
				errorCount++
			}
		}

		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		if exp.Baseline.P95 == 0 {
			continue
		}

		p95 := percentile(latencies, 95)
		errorRate := float64(errorCount) / float64(len(windowed)) * 100

		latencyRatio := float64(p95) / float64(exp.Baseline.P95)
		errorDelta := errorRate - exp.Baseline.ErrorRate

		if latencyRatio > 1.5 || errorDelta > 5 {
			result[ep] = true
		}
	}

	return result
}
