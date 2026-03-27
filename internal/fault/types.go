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
	FaultKillRepeated   FaultType = "kill_repeated"
	FaultNetworkDisable FaultType = "network_disable"
	FaultNetworkEnable  FaultType = "network_enable"
	FaultNetworkFlaky   FaultType = "network_flaky"
	FaultNetworkLatency FaultType = "network_latency"
	FaultNetworkLoss    FaultType = "network_packet_loss"
	FaultRevokeCamera   FaultType = "revoke_camera"
	FaultRevokeStorage  FaultType = "revoke_storage"
	FaultRevokeLocation FaultType = "revoke_location"
	FaultBackgroundApp  FaultType = "background_app"
	FaultForegroundApp  FaultType = "foreground_app"
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
