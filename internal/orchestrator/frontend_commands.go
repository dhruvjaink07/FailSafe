package orchestrator

import (
	"errors"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) GetFrontendFaultCommand(id string) (map[string]interface{}, error) {
	targetType, err := o.GetExperimentTargetType(id)
	if err != nil {
		return nil, err
	}
	if targetType != string(models.TargetFrontend) {
		return nil, errors.New("experiment is not frontend; no web fault command available")
	}

	o.mu.Lock()
	injector := o.injectors[id]
	o.mu.Unlock()

	webInjector, ok := injector.(*fault.WebInjector)
	if !ok {
		return map[string]interface{}{"active": false}, nil
	}

	cmd, active := webInjector.LatestCommand(id)
	if !active {
		return map[string]interface{}{"active": false}, nil
	}

	remaining := time.Until(cmd.ExpiresAt).Milliseconds()
	if remaining < 0 {
		remaining = 0
	}

	return map[string]interface{}{
		"active": true,
		"command": map[string]interface{}{
			"experiment_id":     cmd.ExperimentID,
			"type":              string(cmd.Type),
			"intensity":         cmd.Intensity,
			"targets":           cmd.Targets,
			"issued_at":         cmd.IssuedAt,
			"expires_at":        cmd.ExpiresAt,
			"remaining_time_ms": remaining,
		},
	}, nil
}
