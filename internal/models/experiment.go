package models

import "time"

type DependencyGraph map[string][]string

type ExperimentState string

const (
	StateRunning   ExperimentState = "running"
	StateCompleted ExperimentState = "completed"
	StateFailed    ExperimentState = "failed"
)

type ExperimentPhase string

const (
	PhaseBaseline   ExperimentPhase = "baseline"
	PhaseInjecting  ExperimentPhase = "injecting"
	PhaseRecovering ExperimentPhase = "recovering"
	PhaseCompleted  ExperimentPhase = "completed"
)

type Experiment struct {
	ID string `json:"id"`

	// Observation target (entrypoint of system)
	ObservedEndpoints []string `json:"observed_endpoints"`
	// Injection targets (microservices / containers)
	TargetContainers []string `json:"target_containers"`

	FaultType string `json:"fault_type"`
	Duration  int    `json:"duration_seconds"`

	State ExperimentState `json:"state"`
	Phase ExperimentPhase `json:"phase"`

	FaultStartedAt time.Time `json:"fault_started_at"`
	Intensity      int       `json:"intensity"`
	Adaptive       bool      `json:"adaptive"`
	MaxIntensity   int       `json:"max_intensity"`
	StepIntensity  int       `json:"step_intensity"`

	CurrentIntensity   int   `json:"current_intensity"`
	MaxStableIntensity int   `json:"max_stable_intensity"`
	BreakingIntensity  int   `json:"breaking_intensity"`
	IntensityHistory   []int `json:"intensity_history"`

	TimelineHistory map[int]IntensityTimeline `json:"timeline_history"`
	Baseline        BaselineMetrics           `json:"baseline_metrics"`

	DependencyGraph DependencyGraph `json:"dependency_graph"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type IntensityTimeline struct {
	FaultStartedAt time.Time
	FirstImpact    map[string]time.Time
	RecoveryAt     map[string]time.Time
}

type BaselineMetrics struct {
	AvgLatency float64
	P95        int64
	ErrorRate  float64
}
