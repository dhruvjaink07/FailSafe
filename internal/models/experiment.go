package models

import "time"

type TargetType string

const (
	TargetDocker  TargetType = "docker"
	TargetAndroid TargetType = "android"
)

type DependencyGraph map[string][]string

type GraphMetadata struct {
	TotalNodes int `json:"total_nodes"`
	MaxDepth   int `json:"max_depth"`
}

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
	// Injection targets (services, containers, devices)
	Targets []string `json:"targets"`
	// Target platform (docker | android)
	TargetType string `json:"target_type"`
	// Observation strategy (http | android)
	ObservationType string `json:"observation_type"`

	FaultType string           `json:"fault_type"`
	Duration  int              `json:"duration_seconds"`
	Scenario  []ScheduledFault `json:"scenario,omitempty"`
	Expected  ExpectedState    `json:"expected,omitempty"`
	APKPath   string           `json:"apk_path,omitempty"`
	Package   string           `json:"package,omitempty"`
	Activity  string           `json:"activity,omitempty"`

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

	TargetEndpointMap map[string][]string `json:"target_endpoint_map"`
	GraphMetadata     GraphMetadata       `json:"graph_metadata"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FaultTrigger struct {
	Type           string `json:"type,omitempty"`
	Pattern        string `json:"pattern,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type ExpectedState struct {
	AppState      string `json:"app_state,omitempty"`
	Running       *bool  `json:"running,omitempty"`
	NotCrash      bool   `json:"not_crash,omitempty"`
	NotANR        bool   `json:"not_anr,omitempty"`
	ShouldRecover *bool  `json:"should_recover,omitempty"`
}

type ScheduledFault struct {
	Type            string        `json:"type"`
	At              int           `json:"at"`
	DurationSeconds int           `json:"duration_seconds,omitempty"`
	Intensity       int           `json:"intensity,omitempty"`
	Trigger         *FaultTrigger `json:"trigger,omitempty"`
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
