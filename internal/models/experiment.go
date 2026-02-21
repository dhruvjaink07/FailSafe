package models

import "time"

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

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
