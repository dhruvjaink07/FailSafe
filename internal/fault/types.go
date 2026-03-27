package fault

type FaultType string

const (
	FaultCPU        FaultType = "cpu_stress"
	FaultMemory     FaultType = "memory_stress"
	FaultKill       FaultType = "kill"
	FaultDelay      FaultType = "network_delay"
	FaultPacketLoss FaultType = "packet_loss"

	// android faults
	FaultKillApp        FaultType = "kill_app"
	FaultNetworkDisable FaultType = "network_disable"
	FaultNetworkEnable  FaultType = "network_enable"
	FaultClearData      FaultType = "clear_data"
)

type FaultConfig struct {
	ExperimentID    string
	Targets         []string
	TargetType      string
	Type            FaultType
	DurationSeconds int
	Intensity       int // 1 - 100
}
