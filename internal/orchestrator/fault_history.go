package orchestrator

import (
	"sort"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/fault"
)

type FaultEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

func (o *Orchestrator) recordFaultEvent(id string, faultType fault.FaultType) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.faultHistory[id] = append(o.faultHistory[id], FaultEvent{
		Type:      string(faultType),
		Timestamp: time.Now(),
	})
}

func (o *Orchestrator) getFaultHistory(id string) []FaultEvent {
	o.mu.Lock()
	defer o.mu.Unlock()

	history := append([]FaultEvent(nil), o.faultHistory[id]...)
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})
	return history
}

func (o *Orchestrator) probableCause(id string, firstImpact map[string]time.Time) string {
	history := o.getFaultHistory(id)
	if len(history) == 0 || len(firstImpact) == 0 {
		return ""
	}

	earliestImpact := time.Time{}
	for _, ts := range firstImpact {
		if earliestImpact.IsZero() || ts.Before(earliestImpact) {
			earliestImpact = ts
		}
	}
	if earliestImpact.IsZero() {
		return ""
	}

	cause := ""
	bestAt := time.Time{}
	for _, event := range history {
		if event.Timestamp.After(earliestImpact) {
			continue
		}
		if bestAt.IsZero() || event.Timestamp.After(bestAt) {
			bestAt = event.Timestamp
			cause = event.Type
		}
	}
	return cause
}

func (o *Orchestrator) relativeMillis(start, t time.Time) int64 {
	if start.IsZero() || t.IsZero() {
		return -1
	}
	return t.Sub(start).Milliseconds()
}
