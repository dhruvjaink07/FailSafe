package orchestrator

import (
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
)

func (o *Orchestrator) createCallback(id string) monitoring.EventCallback {
	return func(event monitoring.EventType, sample models.MetricSample) {
		o.mu.Lock()
		defer o.mu.Unlock()

		exp, ok := o.experiments[id]
		if !ok {
			return
		}

		if _, ok := o.metrics[id]; !ok {
			o.metrics[id] = make(map[string][]models.MetricSample)
		}

		o.metrics[id][sample.Endpoint] = append(o.metrics[id][sample.Endpoint], sample)
		o.metricBuffer[id] = append(o.metricBuffer[id], sample)

		now := sample.Timestamp
		if now.IsZero() {
			now = time.Now()
		}

		switch event {
		case monitoring.EventDown:
			if _, exists := o.downtime[id]; !exists {
				o.downtime[id] = now
			}
			if exp.Phase == models.PhaseInjecting {
				if _, exists := o.firstImpact[id][sample.Endpoint]; !exists {
					o.firstImpact[id][sample.Endpoint] = now
				}
			}
		case monitoring.EventDegraded:
			if exp.Phase == models.PhaseInjecting {
				if _, exists := o.firstImpact[id][sample.Endpoint]; !exists {
					o.firstImpact[id][sample.Endpoint] = now
				}
			}
		case monitoring.EventRecovered:
			o.recoveryAt[id][sample.Endpoint] = now
			if downAt, exists := o.downtime[id]; exists {
				dur := now.Sub(downAt)
				o.totalDown[id] += dur
				o.lastRecovery[id] = dur
				o.failures[id]++
				delete(o.downtime, id)
			}
		}
	}
}

func (o *Orchestrator) flushMetricsBatch(id string) error {
	o.mu.Lock()
	batch := append([]models.MetricSample(nil), o.metricBuffer[id]...)
	o.metricBuffer[id] = o.metricBuffer[id][:0]
	o.mu.Unlock()

	if o.db == nil || len(batch) == 0 {
		return nil
	}

	return o.db.InsertMetricsBatch(batch, id)
}

func (o *Orchestrator) startMetricsFlusher(id string) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C

			_ = o.flushMetricsBatch(id)

			o.mu.Lock()
			exp, ok := o.experiments[id]
			if !ok {
				o.mu.Unlock()
				return
			}
			done := exp.State == models.StateCompleted || exp.State == models.StateFailed
			o.mu.Unlock()

			if done {
				_ = o.flushMetricsBatch(id)
				return
			}
		}
	}()
}
