package fault

type FaultType string

const (
	FaultCPU        FaultType = "cpu_stress"
	FaultMemory     FaultType = "memory_stress"
	FaultKill       FaultType = "kill"
	FaultDelay      FaultType = "network_delay"
	FaultPacketLoss FaultType = "packet_loss"
)

type FaultConfig struct {
	ExperimentID    string
	Containers      []string
	Type            FaultType
	DurationSeconds int
	Intensity       int // 1 - 100
}
