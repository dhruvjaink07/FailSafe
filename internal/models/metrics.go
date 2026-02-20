package models

import "time"

type MetricSample struct {
	Timestamp time.Time `json:"timestamp"`

	// HTTP metrics
	CPU       float64 `json:"cpu"`
	LatencyMs int64   `json:"latency_ms"`
	Status    int     `json:"status"`
	IsDown    bool    `json:"is_down"`

	// Container metrics
	ContainerCPU     float64 `json:"container_cpu"`
	ContainerMemMB   float64 `json:"container_mem_mb"`
	ContainerMemPct  float64 `json:"container_mem_percent"`
	ContainerNetIO   string  `json:"container_net_io"`
	ContainerBlockIO string  `json:"container_block_io"`
}
