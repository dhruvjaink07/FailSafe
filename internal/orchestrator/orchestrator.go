package orchestrator

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
	"github.com/google/uuid"
)

type Orchestrator struct {
	experiments  map[string]*models.Experiment
	monitors     map[string]*monitoring.Monitor
	metrics      map[string][]models.MetricSample
	downtime     map[string]time.Time
	totalDown    map[string]time.Duration
	failures     map[string]int
	lastRecovery map[string]time.Duration
	docker       *docker.Manager
	mu           sync.Mutex
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		experiments:  make(map[string]*models.Experiment),
		monitors:     make(map[string]*monitoring.Monitor),
		metrics:      make(map[string][]models.MetricSample),
		downtime:     make(map[string]time.Time),
		totalDown:    make(map[string]time.Duration),
		failures:     make(map[string]int),
		lastRecovery: make(map[string]time.Duration),
		docker:       docker.NewManager(),
	}
}

func (o *Orchestrator) StartExperiment(
	faultType, image, container, portMapping, targetURL string,
	duration int,
) (*models.Experiment, error) {

	if duration <= 0 {
		return nil, errors.New("duration must be greater than 0")
	}

	if faultType == "" || image == "" || container == "" || portMapping == "" || targetURL == "" {
		return nil, errors.New("faultType, image, container, portMapping and targetURL are required")
	}

	// Ensure container ready BEFORE creating experiment
	err := o.docker.EnsureContainerReady(container, image, portMapping)
	if err != nil {
		return nil, errors.New("failed to ensure container ready: " + err.Error())
	}

	id := uuid.New().String()

	exp := &models.Experiment{
		ID:        id,
		FaultType: faultType,
		Image:     image,
		Container: container,
		TargetURL: targetURL,
		Duration:  duration,
		State:     models.StateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	o.mu.Lock()
	o.experiments[id] = exp
	o.metrics[id] = []models.MetricSample{}
	o.totalDown[id] = 0
	o.mu.Unlock()

	// Monitoring callback to track metrics and state changes
	callback := func(event monitoring.EventType, sample models.MetricSample) {

		o.mu.Lock()
		defer o.mu.Unlock()

		// Store metric sample
		o.metrics[id] = append(o.metrics[id], sample)

		switch event {
		case monitoring.EventDown:
			o.downtime[id] = time.Now()
			o.experiments[id].State = models.StateFaulted
			o.experiments[id].UpdatedAt = time.Now()
			o.failures[id]++

		case monitoring.EventRecovered:
			if start, ok := o.downtime[id]; ok {
				recoveryTime := time.Since(start)
				o.totalDown[id] += recoveryTime
				o.lastRecovery[id] = recoveryTime
				delete(o.downtime, id)
			}

			o.completeExperimentLocked(id)
		}
	}
	monitor := monitoring.NewMonitor(callback, o.docker, container)

	o.mu.Lock()
	o.monitors[id] = monitor
	o.mu.Unlock()

	monitor.Start(id, targetURL)

	// Duration cap
	go func() {
		time.Sleep(time.Duration(duration) * time.Second)

		o.mu.Lock()
		defer o.mu.Unlock()

		experiment, exists := o.experiments[id]
		if !exists {
			return
		}

		if experiment.State != models.StateCompleted {
			o.completeExperimentLocked(id)
		}
	}()

	return exp, nil
}

// StopExperiment allows manual stopping
func (o *Orchestrator) StopExperiment(id string) error {

	o.mu.Lock()
	defer o.mu.Unlock()

	experiment, exists := o.experiments[id]
	if !exists {
		return errors.New("experiment not found")
	}

	if experiment.State == models.StateCompleted {
		return nil
	}

	o.completeExperimentLocked(id)
	return nil
}

// completeExperimentLocked assumes mutex already locked
func (o *Orchestrator) completeExperimentLocked(id string) {
	exp := o.experiments[id]
	exp.State = models.StateCompleted
	exp.UpdatedAt = time.Now()

	if monitor, ok := o.monitors[id]; ok {
		monitor.Stop()
		delete(o.monitors, id)
	}

	// Stop container safely
	if exp.Container != "" {
		_ = o.docker.StopContainer(exp.Container)
	}
}

func (o *Orchestrator) GetExperiment(id string) (*models.Experiment, error) {

	o.mu.Lock()
	defer o.mu.Unlock()

	exp, exists := o.experiments[id]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	// Return a copy to avoid external mutation
	copyExp := *exp
	return &copyExp, nil
}

func (o *Orchestrator) GetMetrics(id string) (interface{}, error) {

	o.mu.Lock()
	defer o.mu.Unlock()

	samples, exists := o.metrics[id]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	totalRequests := len(samples)
	if totalRequests == 0 {
		return nil, errors.New("no metrics collected")
	}

	var latencies []int64
	var cpuSeries []float64
	var containerCPUSeries []float64

	var errorCount int
	var count4xx int
	var count5xx int

	var totalContainerCPU float64
	var maxContainerCPU float64
	var totalMem float64
	var maxMem float64

	var currentFailureStreak int
	var maxFailureStreak int

	for _, s := range samples {

		latencies = append(latencies, s.LatencyMs)
		cpuSeries = append(cpuSeries, s.CPU)
		containerCPUSeries = append(containerCPUSeries, s.ContainerCPU)

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
		totalMem += s.ContainerMemMB

		if s.ContainerCPU > maxContainerCPU {
			maxContainerCPU = s.ContainerCPU
		}
		if s.ContainerMemMB > maxMem {
			maxMem = s.ContainerMemMB
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

	exp := o.experiments[id]
	endTime := time.Now()
	if exp.State == models.StateCompleted {
		endTime = exp.UpdatedAt
	}

	totalDuration := endTime.Sub(exp.CreatedAt).Seconds()
	throughput := float64(totalRequests) / totalDuration

	uptimePercent := 100.0
	if totalDuration > 0 {
		uptimePercent = 100 - (o.totalDown[id].Seconds()/totalDuration)*100
	}

	var mttr float64
	if o.failures[id] > 0 {
		mttr = o.totalDown[id].Seconds() / float64(o.failures[id])
	}

	avgContainerCPU := totalContainerCPU / float64(totalRequests)
	avgMem := totalMem / float64(totalRequests)

	// Drift calculation
	firstSegment := latencies[:len(latencies)/3]
	lastSegment := latencies[len(latencies)*2/3:]
	driftRatio := meanInt64(lastSegment) / meanInt64(firstSegment)

	// Correlation between container CPU and latency
	var latencyFloat []float64
	for _, l := range latencies {
		latencyFloat = append(latencyFloat, float64(l))
	}
	cpuLatencyCorrelation := correlation(containerCPUSeries, latencyFloat)

	// Stability score
	stabilityScore := 100.0
	stabilityScore -= errorRate * 0.5
	stabilityScore -= (latencyStd / float64(avgLatency)) * 20
	stabilityScore -= (100 - uptimePercent) * 0.5
	if stabilityScore < 0 {
		stabilityScore = 0
	}

	availabilityClass := "unstable"
	if uptimePercent >= 99.9 {
		availabilityClass = "five_nines"
	} else if uptimePercent >= 99 {
		availabilityClass = "highly_available"
	}

	return map[string]interface{}{
		"requests_total": totalRequests,
		"throughput_rps": throughput,

		"latency": map[string]interface{}{
			"avg_ms":      avgLatency,
			"p50_ms":      p50,
			"p95_ms":      p95,
			"p99_ms":      p99,
			"stddev_ms":   latencyStd,
			"jitter_ms":   latencyJitter,
			"drift_ratio": driftRatio,
		},

		"errors": map[string]interface{}{
			"total":              errorCount,
			"rate_percent":       errorRate,
			"4xx":                count4xx,
			"5xx":                count5xx,
			"max_failure_streak": maxFailureStreak,
		},

		"reliability": map[string]interface{}{
			"downtime_seconds":      o.totalDown[id].Seconds(),
			"uptime_percent":        uptimePercent,
			"failure_count":         o.failures[id],
			"mttr_seconds":          mttr,
			"last_recovery_seconds": o.lastRecovery[id].Seconds(),
			"availability_class":    availabilityClass,
		},

		"container": map[string]interface{}{
			"avg_cpu_percent":         avgContainerCPU,
			"max_cpu_percent":         maxContainerCPU,
			"avg_memory_mb":           avgMem,
			"max_memory_mb":           maxMem,
			"cpu_saturation":          maxContainerCPU > 80,
			"memory_pressure":         maxMem > 500,
			"cpu_latency_correlation": cpuLatencyCorrelation,
		},

		"stability_score": stabilityScore,
		"samples":         samples,
	}, nil
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
