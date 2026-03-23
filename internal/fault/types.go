package fault

type FaultType string

const (
	FaultCPU              FaultType = "cpu_stress"
	FaultMemory           FaultType = "memory_stress"
	FaultKill             FaultType = "kill"
	FaultDelay            FaultType = "network_delay"
	FaultPacketLoss       FaultType = "packet_loss"
	FaultAppKill          FaultType = "app_kill"
	FaultNetworkToggle    FaultType = "network_toggle"
	FaultPermissionRevoke FaultType = "permission_revoke"
	FaultRotation         FaultType = "rotation"
)

type FaultConfig struct {
	ExperimentID    string
	Targets         []string
	TargetType      string
	Type            FaultType
	DurationSeconds int
	Intensity       int // 1 - 100
}
