package models

import "time"

// This defines lifecycle of states

type ExperimentState string

const (
	StateCreated    ExperimentState = "created"
	StateRunning    ExperimentState = "running"
	StateFaulted    ExperimentState = "faulted"
	StateRecovering ExperimentState = "recovering"
	StateCompleted  ExperimentState = "completed"
	StateFailed     ExperimentState = "failed"
)

// This is our in-memory Representation of an experiment. In a real implementation, this would likely be stored in a database.
type Experiment struct {
	ID        string
	FaultType string
	Target    string
	Duration  int

	State     ExperimentState
	CreatedAt time.Time
	UpdatedAt time.Time
}
