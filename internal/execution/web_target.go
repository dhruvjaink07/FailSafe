package execution

import (
	"github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
)

// WebTarget is the execution adapter for browser/web resilience experiments.
// It receives pluggable monitor/injector implementations so web behavior can
// evolve without changing orchestrator contract shape.
type WebTarget struct {
	*BaseTarget
}

func NewWebTarget(injector fault.Injector, monitor monitoring.MonitorInterface) *WebTarget {
	if injector == nil {
		injector = fault.NewNoopInjector()
	}
	if monitor == nil {
		monitor = monitoring.NewNoopMonitor()
	}

	return &WebTarget{
		BaseTarget: newBaseTarget(models.TargetFrontend, injector, monitor),
	}
}
