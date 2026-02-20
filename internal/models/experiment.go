package models

import "time"

/*
---------------------------------------
Experiment State Machine
---------------------------------------
*/

type ExperimentState string

const (
	StateCreated    ExperimentState = "created"
	StateRunning    ExperimentState = "running"
	StateFaulted    ExperimentState = "faulted"
	StateRecovering ExperimentState = "recovering"
	StateCompleted  ExperimentState = "completed"
	StateFailed     ExperimentState = "failed"
)

/*
---------------------------------------
Experiment Model
---------------------------------------

Represents a single fault injection experiment.

This struct is the single source of truth
for experiment lifecycle and metadata.
*/

type Experiment struct {

	// Unique experiment ID
	ID string `json:"id"`

	// Type of fault being injected
	FaultType string `json:"fault_type"`

	// Docker image used for this experiment
	Image string `json:"image"`

	// Docker container name used for testing
	Container string `json:"container"`

	// Target URL monitored for health
	TargetURL string `json:"target_url"`

	// Duration in seconds
	Duration int `json:"duration"`

	// Current lifecycle state
	State ExperimentState `json:"state"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
