package fault

type FaultType string

const (
	FaultCPU    FaultType = "cpu_stress"
	FaultMemory FaultType = "memory_stress"
	FaultKill  FaultType = "kill"
	FaultDelay FaultType = "network_delay"
)

type FaultConfig struct {
	ExperimentID string
	Containers  []string
	Type   FaultType
	DurationSeconds int
}

type Injector struct {
}

func (i *Injector) Inject(config FaultConfig) error {
	return nil
}