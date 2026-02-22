package orchestrator

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/fault"

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

	mu sync.Mutex
}

func NewOrchestrator() *Orchestrator {

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
		injector:     fault.NewMockInjector(dm), // Replace with RustInjector later
	}
}

func (o *Orchestrator) StartExperiment(
	faultType string,
	targetContainers []string,
	observedEndpoints []string,
	duration int,
) (*models.Experiment, error) {

	if duration <= 0 {
		return nil, errors.New("duration must be greater than 0")
	}

	if len(targetContainers) == 0 || len(observedEndpoints) == 0 {
		return nil, errors.New("targetContainers and observedEndpoints are required")
	}

	// Ensure all target containers are ready
	for _, c := range targetContainers {
		if err := o.docker.EnsureContainerReady(c, "", ""); err != nil {
			return nil, err
		}
	}

	id := uuid.New().String()

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

	callback := func(event monitoring.EventType, sample models.MetricSample) {

		o.mu.Lock()
		defer o.mu.Unlock()

		o.metrics[id][sample.Endpoint] = append(o.metrics[id][sample.Endpoint], sample)

		switch event {

		case monitoring.EventDown:
			if _, exists := o.firstImpact[id][sample.Endpoint]; !exists {
				fmt.Println("First Impact At ", time.Now())
				o.firstImpact[id][sample.Endpoint] = time.Now()
			}
			if _, exists := o.downtime[id]; !exists {
				o.downtime[id] = time.Now()
				o.failures[id]++
			}

		case monitoring.EventRecovered:
			if _, impacted := o.firstImpact[id][sample.Endpoint]; impacted {
				if _, recorded := o.recoveryAt[id][sample.Endpoint]; !recorded {
					fmt.Println("Recovered At ", time.Now())
					o.recoveryAt[id][sample.Endpoint] = time.Now()
				}
			}
			if start, ok := o.downtime[id]; ok {

				// Only clear downtime when ALL endpoints recovered
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

	go o.runTimeline(id)

	return exp, nil
}

func (o *Orchestrator) runTimeline(id string) {

	// ---- BASELINE ----
	o.setPhase(id, models.PhaseBaseline)
	time.Sleep(5 * time.Second)

	// ---- INJECTION ----
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
	}

	_ = o.injector.Inject(config)

	// ---- RECOVERY PHASE ----
	o.setPhase(id, models.PhaseRecovering)

	// Give monitor enough time to detect recovery
	time.Sleep(time.Duration(exp.Duration/2) * time.Second)

	// Extra buffer to ensure recovery detection
	time.Sleep(3 * time.Second)

	o.completeExperiment(id)
}
func (o *Orchestrator) setPhase(id string, phase models.ExperimentPhase) {

	o.mu.Lock()
	defer o.mu.Unlock()

	exp, ok := o.experiments[id]
	if !ok {
		return
	}

	exp.Phase = phase
	exp.UpdatedAt = time.Now()
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
	defer o.mu.Unlock()
	o.completeExperimentLocked(id)
}

func (o *Orchestrator) completeExperimentLocked(id string) {

	exp := o.experiments[id]
	exp.State = models.StateCompleted
	exp.Phase = models.PhaseCompleted
	exp.UpdatedAt = time.Now()

	if monitor, ok := o.monitors[id]; ok {
		monitor.Stop()
		delete(o.monitors, id)
	}
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
	defer o.mu.Unlock()

	exp, exists := o.experiments[id]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	endpointMap, exists := o.metrics[id]
	if !exists {
		return nil, errors.New("no metrics found")
	}

	result := make(map[string]interface{})
	endpointResults := make(map[string]interface{})

	totalEndpoints := 0
	degradedEndpoints := 0
	globalRequests := 0

	for endpoint, samples := range endpointMap {

		if len(samples) == 0 {
			continue
		}

		totalEndpoints++
		totalRequests := len(samples)
		globalRequests += totalRequests

		var latencies []int64
		var totalContainerCPU float64
		var maxContainerCPU float64
		var totalMem float64
		var maxMem float64

		var errorCount int
		var count4xx int
		var count5xx int
		var currentFailureStreak int
		var maxFailureStreak int

		for _, s := range samples {

			latencies = append(latencies, s.LatencyMs)

			if s.Status >= 400 || s.Status == 0 {
				errorCount++
				currentFailureStreak++
				if currentFailureStreak > maxFailureStreak {
					maxFailureStreak = currentFailureStreak
				}
			} else {
				currentFailureStreak = 0
			}

			if s.Status >= 400 && s.Status < 500 {
				count4xx++
			}
			if s.Status >= 500 {
				count5xx++
			}

			totalContainerCPU += s.ContainerCPU
			totalMem += s.ContainerMemoryMB

			if s.ContainerCPU > maxContainerCPU {
				maxContainerCPU = s.ContainerCPU
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
		latencyStd := stddev(latencies, avgLatency)
		latencyJitter := jitter(latencies)

		errorRate := float64(errorCount) / float64(totalRequests) * 100

		expDuration := exp.UpdatedAt.Sub(exp.CreatedAt).Seconds()
		if expDuration <= 0 {
			expDuration = 1
		}

		throughput := float64(totalRequests) / expDuration

		uptimePercent := 100.0
		if expDuration > 0 {
			uptimePercent = 100 - (o.totalDown[id].Seconds()/expDuration)*100
		}

		avgContainerCPU := totalContainerCPU / float64(totalRequests)
		avgMem := totalMem / float64(totalRequests)

		stabilityScore := 100.0
		stabilityScore -= errorRate * 0.5
		if avgLatency > 0 {
			stabilityScore -= (latencyStd / float64(avgLatency)) * 20
		}
		stabilityScore -= (100 - uptimePercent) * 0.5
		if stabilityScore < 0 {
			stabilityScore = 0
		}

		degraded := errorRate > 10 || p95 > avgLatency*2
		if degraded {
			degradedEndpoints++
		}

		// ---------------- Timeline Calculation ----------------

		faultStart := exp.FaultStartedAt

		var propagationDelay float64
		var recoveryDelay float64

		if impactTime, ok := o.firstImpact[id][endpoint]; ok && !faultStart.IsZero() {
			propagationDelay = impactTime.Sub(faultStart).Seconds()
		}

		if recoveryTime, ok := o.recoveryAt[id][endpoint]; ok {
			if impactTime, ok2 := o.firstImpact[id][endpoint]; ok2 {
				recoveryDelay = recoveryTime.Sub(impactTime).Seconds()
			}
		}

		endpointResults[endpoint] = map[string]interface{}{
			"requests_total": totalRequests,
			"throughput_rps": throughput,
			"latency": map[string]interface{}{
				"avg_ms":    avgLatency,
				"p50_ms":    p50,
				"p95_ms":    p95,
				"p99_ms":    p99,
				"stddev_ms": latencyStd,
				"jitter_ms": latencyJitter,
			},
			"errors": map[string]interface{}{
				"total":              errorCount,
				"rate_percent":       errorRate,
				"4xx":                count4xx,
				"5xx":                count5xx,
				"max_failure_streak": maxFailureStreak,
			},
			"container": map[string]interface{}{
				"avg_cpu_percent": avgContainerCPU,
				"max_cpu_percent": maxContainerCPU,
				"avg_memory_mb":   avgMem,
				"max_memory_mb":   maxMem,
			},
			"stability_score": stabilityScore,
			"degraded":        degraded,
			"timeline": map[string]interface{}{
				"fault_started_at":      faultStart,
				"propagation_delay_sec": propagationDelay,
				"recovery_delay_sec":    recoveryDelay,
			},
		}
	}

	blastRadius := 0.0
	if totalEndpoints > 0 {
		blastRadius = float64(degradedEndpoints) / float64(totalEndpoints) * 100
	}

	result["endpoints"] = endpointResults
	result["blast_radius_percent"] = blastRadius
	result["total_requests"] = globalRequests
	result["experiment_state"] = exp.State
	result["experiment_phase"] = exp.Phase

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
