package fault

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type WebCommand struct {
	ExperimentID string
	Type         FaultType
	Intensity    int
	Targets      []string
	IssuedAt     time.Time
	ExpiresAt    time.Time
}

// WebInjector acts as a control plane for browser faults.
// Browser automation workers can read latest commands per experiment.
type WebInjector struct {
	mu       sync.RWMutex
	commands map[string]WebCommand
}

func NewWebInjector() *WebInjector {
	return &WebInjector{commands: make(map[string]WebCommand)}
}

func (w *WebInjector) Inject(config FaultConfig) error {
	if strings.TrimSpace(config.ExperimentID) == "" {
		return errors.New("missing experiment id")
	}
	targetType := strings.ToLower(strings.TrimSpace(config.TargetType))
	if targetType != "frontend" && targetType != "web" {
		return errors.New("web injector requires target_type frontend/web")
	}
	if !isSupportedWebFault(config.Type) {
		return errors.New("unsupported web fault type")
	}
	dur := config.DurationSeconds
	if dur <= 0 {
		dur = 1
	}

	now := time.Now()
	cmd := WebCommand{
		ExperimentID: config.ExperimentID,
		Type:         config.Type,
		Intensity:    config.Intensity,
		Targets:      append([]string(nil), config.Targets...),
		IssuedAt:     now,
		ExpiresAt:    now.Add(time.Duration(dur) * time.Second),
	}

	w.mu.Lock()
	w.commands[config.ExperimentID] = cmd
	w.mu.Unlock()

	return nil
}

func (w *WebInjector) LatestCommand(experimentID string) (WebCommand, bool) {
	w.mu.RLock()
	cmd, ok := w.commands[experimentID]
	w.mu.RUnlock()
	if !ok {
		return WebCommand{}, false
	}
	if time.Now().After(cmd.ExpiresAt) {
		return WebCommand{}, false
	}
	return cmd, true
}

func isSupportedWebFault(t FaultType) bool {
	switch strings.ToLower(strings.TrimSpace(string(t))) {
	case "network_delay", "network_latency", "network_packet_loss", "packet_loss", "cpu_throttle", "offline", "request_abort", "js_chaos":
		return true
	default:
		return false
	}
}
