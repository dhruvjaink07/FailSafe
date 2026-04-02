package execution

import (
	"context"
	"errors"
	"sync"

	"github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
)

// Target defines a single execution domain contract used by docker, android, and web.
// Orchestrator-facing behavior stays uniform across all runtime types.
type Target interface {
	Kind() models.TargetType
	Start(ctx context.Context, experimentID string, observedEndpoints []string) error
	Inject(config fault.FaultConfig) error
	SetIntensity(int)
	Stop() error
}

// BaseTarget provides a concurrency-safe default implementation that delegates
// monitoring and fault injection to domain-specific components.
type BaseTarget struct {
	kind     models.TargetType
	injector fault.Injector
	monitor  monitoring.MonitorInterface

	mu      sync.Mutex
	started bool
}

func newBaseTarget(kind models.TargetType, injector fault.Injector, monitor monitoring.MonitorInterface) *BaseTarget {
	return &BaseTarget{
		kind:     kind,
		injector: injector,
		monitor:  monitor,
	}
}

func (t *BaseTarget) Kind() models.TargetType {
	return t.kind
}

func (t *BaseTarget) Start(_ context.Context, experimentID string, observedEndpoints []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return errors.New("target already started")
	}
	if t.monitor == nil {
		return errors.New("monitor is not configured")
	}

	t.monitor.Start(experimentID, observedEndpoints)
	t.started = true
	return nil
}

func (t *BaseTarget) Inject(config fault.FaultConfig) error {
	if t.injector == nil {
		return errors.New("injector is not configured")
	}
	return t.injector.Inject(config)
}

func (t *BaseTarget) SetIntensity(intensity int) {
	if t.monitor != nil {
		t.monitor.SetIntensity(intensity)
	}
}

func (t *BaseTarget) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.started {
		return nil
	}
	if t.monitor == nil {
		return errors.New("monitor is not configured")
	}

	t.monitor.Stop()
	t.started = false
	return nil
}
