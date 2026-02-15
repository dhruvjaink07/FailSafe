package models

import "time"

type MetricSample struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu"`
	LatencyMs int64     `json:"latency_ms"`
	Status    int       `json:"status"`
	IsDown    bool      `json:"is_down"`
}
