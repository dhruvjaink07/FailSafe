package models

import "time"

type MetricSample struct {
	Timestamp time.Time `json:"timestamp"`
	Endpoint  string    `json:"endpoint"`

	// System-level metrics
	CPU       float64 `json:"cpu_percent"`
	LatencyMs int64   `json:"latency_ms"`
	Status    int     `json:"status_code"`
	IsDown    bool    `json:"is_down"`

	// Container-level metrics (optional)
	ContainerCPU        float64 `json:"container_cpu_percent"`
	ContainerMemoryMB   float64 `json:"container_memory_mb"`
	ContainerMemPercent float64 `json:"container_memory_percent"`
	ContainerNetIO      string  `json:"container_net_io"`
	ContainerBlockIO    string  `json:"container_block_io"`
}
