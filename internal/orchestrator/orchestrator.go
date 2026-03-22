package orchestrator

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/storage"

	// "github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
	"github.com/google/uuid"
)

type Orchestrator struct {
	experiments map[string]*models.Experiment
	monitors    map[string]*monitoring.Monitor
	metrics     map[string]map[string][]models.MetricSample

	downtime     map[string]time.Time
	totalDown    map[string]time.Duration
	failures     map[string]int
	lastRecovery map[string]time.Duration

	docker      *docker.Manager
	injector    *fault.MockInjector
	firstImpact map[string]map[string]time.Time
	recoveryAt  map[string]map[string]time.Time

	db           *storage.Postgres
	metricBuffer map[string][]models.MetricSample
	mu           sync.Mutex
}

func NewOrchestrator(db *storage.Postgres) *Orchestrator {

	dm := docker.NewManager()

	return &Orchestrator{
		experiments:  make(map[string]*models.Experiment),
		monitors:     make(map[string]*monitoring.Monitor),
		metrics:      make(map[string]map[string][]models.MetricSample),
		downtime:     make(map[string]time.Time),
		totalDown:    make(map[string]time.Duration),
		failures:     make(map[string]int),
		lastRecovery: make(map[string]time.Duration),
		firstImpact:  make(map[string]map[string]time.Time),
		recoveryAt:   make(map[string]map[string]time.Time),
		docker:       dm,
		injector:     fault.NewMockInjector(dm),
		db:           db,
		metricBuffer: make(map[string][]models.MetricSample),
	}
}

func (o *Orchestrator) StartExperiment(
	faultType string,
	targetContainers []string,
	observedEndpoints []string,
	duration int,
	adaptive bool,
	stepIntensity int,
	maxIntensity int,
	deps models.DependencyGraph,
	containerMap map[string][]string,
) (*models.Experiment, error) {

	if duration <= 0 {
		return nil, errors.New("duration must be greater than 0")
	}

	if len(targetContainers) == 0 || len(observedEndpoints) == 0 {
		return nil, errors.New("targetContainers and observedEndpoints are required")
	}

	for _, c := range targetContainers {
		if err := o.docker.EnsureContainerReady(c, "", ""); err != nil {
			return nil, err
		}
	}

	id := uuid.New().String()

	meta := computeGraphMeta(deps)

	exp := &models.Experiment{
		ID:                id,
		ObservedEndpoints: observedEndpoints,
		TargetContainers:  targetContainers,
		FaultType:         faultType,
		Duration:          duration,
		State:             models.StateRunning,
		Phase:             models.PhaseBaseline,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),

		Adaptive:             adaptive,
		StepIntensity:        stepIntensity,
		MaxIntensity:         maxIntensity,
		IntensityHistory:     []int{},
		MaxStableIntensity:   0,
		BreakingIntensity:    0,
		TimelineHistory:      make(map[int]models.IntensityTimeline),
		DependencyGraph:      deps,
		GraphMetadata:        meta,
		ContainerEndpointMap: containerMap,
	}

	o.mu.Lock()
	o.experiments[id] = exp
	o.metrics[id] = make(map[string][]models.MetricSample)
	for _, ep := range observedEndpoints {
		o.metrics[id][ep] = []models.MetricSample{}
	}
	o.totalDown[id] = 0
	o.failures[id] = 0
	o.firstImpact[id] = make(map[string]time.Time)
	o.recoveryAt[id] = make(map[string]time.Time)
	o.mu.Unlock()

	if o.db != nil {
		err := o.db.InsertExperiment(exp)
		if err != nil {
			fmt.Println("DB insert failed:", err)
		}
	}
	callback := func(event monitoring.EventType, sample models.MetricSample) {

		o.mu.Lock()
		defer o.mu.Unlock()

		o.metrics[id][sample.Endpoint] =
			append(o.metrics[id][sample.Endpoint], sample)
		o.metricBuffer[id] = append(o.metricBuffer[id], sample)
		switch event {

		case monitoring.EventDegraded:
			if _, exists := o.firstImpact[id][sample.Endpoint]; !exists {
				o.firstImpact[id][sample.Endpoint] = time.Now()
			}

		case monitoring.EventDown:
			if _, exists := o.downtime[id]; !exists {
				o.downtime[id] = time.Now()
				o.failures[id]++
			}

		case monitoring.EventRecovered:

			if _, impacted := o.firstImpact[id][sample.Endpoint]; impacted {
				if _, recorded := o.recoveryAt[id][sample.Endpoint]; !recorded {
					o.recoveryAt[id][sample.Endpoint] = time.Now()
				}
			}

			if start, ok := o.downtime[id]; ok {

				allRecovered := true
				for ep := range o.firstImpact[id] {
					if _, recovered := o.recoveryAt[id][ep]; !recovered {
						allRecovered = false
						break
					}
				}

				if allRecovered {
					recoveryTime := time.Since(start)
					o.totalDown[id] += recoveryTime
					o.lastRecovery[id] = recoveryTime
					delete(o.downtime, id)
				}
			}
		}
	}

	monitor := monitoring.NewMonitor(callback, o.docker, targetContainers)

	o.mu.Lock()
	o.monitors[id] = monitor
	o.mu.Unlock()

	monitor.Start(id, observedEndpoints)

	o.startMetricsFlusher(id)

	go o.runTimeline(id)

	return exp, nil
}

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

		// RESET FIRST (fix for timeline overwrite issue)
		o.firstImpact[id] = make(map[string]time.Time)
		o.recoveryAt[id] = make(map[string]time.Time)
		delete(o.downtime, id)

		exp = o.experiments[id] // FIX: no shadowing

		exp.CurrentIntensity = mid
		exp.IntensityHistory = append(exp.IntensityHistory, mid)
		exp.FaultStartedAt = startTime

		if monitor, ok := o.monitors[id]; ok {
			monitor.SetIntensity(mid)
		}

		o.mu.Unlock()

		config := fault.FaultConfig{
			ExperimentID:    id,
			Containers:      exp.TargetContainers,
			Type:            fault.FaultType(exp.FaultType),
			DurationSeconds: 5,
			Intensity:       mid,
		}

		_ = o.injector.Inject(config)

		time.Sleep(10 * time.Second)

		// snapshot timeline
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

		// ---- DECISION PHASE ----

		degraded := o.isExperimentDegraded(id, startTime)
		degradedMap := o.getDegradedEndpoints(id, startTime)

		o.mu.Lock()
		exp = o.experiments[id] // FIX: reuse reference
		o.mu.Unlock()

		blast, depth := o.computeGraphImpactScore(exp, degradedMap)

		// strict local degradation
		if degraded {
			breaking = mid
			high = mid - 1
			continue
		}

		// graph-aware degradation
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

func (o *Orchestrator) setPhase(id string, phase models.ExperimentPhase) {

	o.mu.Lock()

	exp, ok := o.experiments[id]
	if !ok {
		o.mu.Unlock()
		return
	}

	exp.Phase = phase
	exp.UpdatedAt = time.Now()

	o.mu.Unlock()

	// optional persistence (low cost)
	if o.db != nil {
		_ = o.db.UpdateExperimentResults(exp)
	}
}

func (o *Orchestrator) StopExperiment(id string) error {

	o.mu.Lock()
	defer o.mu.Unlock()

	exp, exists := o.experiments[id]
	if !exists {
		return errors.New("experiment not found")
	}

	if exp.State == models.StateCompleted {
		return nil
	}

	o.completeExperimentLocked(id)
	return nil
}

func (o *Orchestrator) completeExperiment(id string) {

	o.mu.Lock()
	o.completeExperimentLocked(id)
	exp := o.experiments[id]
	o.mu.Unlock()

	if o.db != nil {
		err := o.db.UpdateExperimentResults(exp)
		if err != nil {
			fmt.Println("DB update failed:", err)
		}
	}

	// -------- AGGREGATION --------

	metrics, err := o.GetMetrics(id)
	if err != nil {
		fmt.Println("aggregation failed:", err)
		return
	}

	data := metrics.(map[string]interface{})

	if o.db != nil {

		err = o.db.InsertAggregatedMetrics(id, data)
		if err != nil {
			fmt.Println("agg insert error:", err)
		}

		err = o.db.InsertExperimentSummary(id, data)
		if err != nil {
			fmt.Println("summary insert error:", err)
		}
	}
}

func (o *Orchestrator) completeExperimentLocked(id string) {

	exp := o.experiments[id]
	exp.State = models.StateCompleted
	exp.Phase = models.PhaseCompleted
	exp.UpdatedAt = time.Now()

	if monitor, ok := o.monitors[id]; ok {
		monitor.Stop()
		delete(o.monitors, id)
		delete(o.metricBuffer, id)
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
			if s.Timestamp.After(since) &&
				s.Intensity == exp.CurrentIntensity {
				windowed = append(windowed, s)
			}
		}

		if len(windowed) < 5 {
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
func (o *Orchestrator) runStaticFault(id string) {

	o.setPhase(id, models.PhaseInjecting)

	o.mu.Lock()
	exp := o.experiments[id]
	exp.FaultStartedAt = time.Now()
	o.mu.Unlock()

	config := fault.FaultConfig{
		ExperimentID:    id,
		Containers:      exp.TargetContainers,
		Type:            fault.FaultType(exp.FaultType),
		DurationSeconds: exp.Duration / 2,
		Intensity:       exp.Intensity,
	}

	_ = o.injector.Inject(config)

	o.setPhase(id, models.PhaseRecovering)
	time.Sleep(5 * time.Second)

	o.completeExperiment(id)

}

func (o *Orchestrator) GetExperiment(id string) (*models.Experiment, error) {

	o.mu.Lock()
	defer o.mu.Unlock()

	exp, exists := o.experiments[id]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	copyExp := *exp
	return &copyExp, nil
}

func (o *Orchestrator) GetMetrics(id string) (interface{}, error) {

	o.mu.Lock()
	exp, exists := o.experiments[id]
	if !exists {
		o.mu.Unlock()
		return nil, errors.New("experiment not found")
	}

	endpointMap, exists := o.metrics[id]
	if !exists {
		o.mu.Unlock()
		return nil, errors.New("no metrics found")
	}
	o.mu.Unlock()

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

		// -------- DERIVED --------

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

		degraded := latencyRatio > 1.5 || errorDelta > 5
		if degraded {
			degradedMap[endpoint] = true
		}

		// -------- IMPACT ORDER --------

		impactOrder := 0

		selectedIntensity := exp.BreakingIntensity
		if selectedIntensity == 0 {
			selectedIntensity = exp.MaxStableIntensity
		}

		if timelineData, ok := exp.TimelineHistory[selectedIntensity]; ok {

			if t, exists := timelineData.FirstImpact[endpoint]; exists {

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

	result["resilience_threshold"] = map[string]interface{}{
		"max_stable_intensity": exp.MaxStableIntensity,
		"breaking_intensity":   exp.BreakingIntensity,
		"intensity_steps":      exp.IntensityHistory,
	}

	return result, nil
}

func percentile(data []int64, p int) int64 {
	sort.Slice(data, func(i, j int) bool {
		return data[i] < data[j]
	})

	index := (len(data) - 1) * p / 100
	return data[index]
}

func stddev(data []int64, mean int64) float64 {
	var variance float64
	for _, v := range data {
		diff := float64(v - mean)
		variance += diff * diff
	}
	variance /= float64(len(data))
	return sqrt(variance)
}

func sqrt(value float64) float64 {
	z := value
	for i := 0; i < 10; i++ {
		z -= (z*z - value) / (2 * z)
	}
	return z
}

func meanInt64(data []int64) float64 {
	var sum int64
	for _, v := range data {
		sum += v
	}
	return float64(sum) / float64(len(data))
}

func jitter(data []int64) float64 {
	if len(data) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(data); i++ {
		diff := data[i] - data[i-1]
		if diff < 0 {
			diff = -diff
		}
		total += float64(diff)
	}
	return total / float64(len(data)-1)
}

func correlation(x []float64, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	n := float64(len(x))

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	num := (n * sumXY) - (sumX * sumY)
	den := sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if den == 0 {
		return 0
	}
	return num / den
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

// Handling Dependency Graph
func (o *Orchestrator) computeCascadeDepth(exp *models.Experiment, degraded map[string]bool) int {

	maxDepth := 0

	var dfs func(node string, depth int, visited map[string]bool)

	dfs = func(node string, depth int, visited map[string]bool) {

		if visited[node] {
			return
		}

		visited[node] = true

		if depth > maxDepth {
			maxDepth = depth
		}

		for _, dep := range exp.DependencyGraph[node] {
			if degraded[dep] {
				dfs(dep, depth+1, visited)
			}
		}
	}

	for node := range degraded {
		visited := make(map[string]bool)
		dfs(node, 1, visited)
	}

	return maxDepth
}

func computeGraphMeta(graph models.DependencyGraph) models.GraphMetadata {
	visited := make(map[string]bool)
	maxDepth := 0

	var dfs func(string, int)
	dfs = func(node string, depth int) {
		if visited[node] {
			return
		}
		visited[node] = true

		if depth > maxDepth {
			maxDepth = depth
		}

		for _, n := range graph[node] {
			dfs(n, depth+1)
		}
	}

	for node := range graph {
		dfs(node, 1)
	}

	return models.GraphMetadata{
		TotalNodes: len(visited),
		MaxDepth:   maxDepth,
	}
}

func getImpactedEndpoints(
	containers []string,
	containerMap map[string][]string,
) []string {

	seen := make(map[string]bool)
	var result []string

	for _, c := range containers {
		for _, ep := range containerMap[c] {
			if !seen[ep] {
				seen[ep] = true
				result = append(result, ep)
			}
		}
	}

	return result
}

func (o *Orchestrator) computeGraphImpactScore(
	exp *models.Experiment,
	degraded map[string]bool,
) (float64, int) {

	total := exp.GraphMetadata.TotalNodes
	if total == 0 {
		return 0, 0
	}

	affected := len(degraded)
	blast := float64(affected) / float64(total) * 100
	depth := o.computeCascadeDepth(exp, degraded)

	return blast, depth
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
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
			if s.Timestamp.After(since) &&
				s.Intensity == exp.CurrentIntensity {
				windowed = append(windowed, s)
			}
		}

		if len(windowed) < 5 {
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

func (o *Orchestrator) startMetricsFlusher(id string) {

	ticker := time.NewTicker(2 * time.Second)

	go func() {
		for range ticker.C {

			o.mu.Lock()

			buffer := o.metricBuffer[id]
			if len(buffer) == 0 {
				o.mu.Unlock()
				continue
			}

			o.metricBuffer[id] = nil

			o.mu.Unlock()

			if o.db != nil {
				err := o.db.InsertMetricsBatch(buffer, id)
				if err != nil {
					fmt.Println("metrics batch insert error:", err)
				}
			}
		}
	}()
}
