package monitoring

type NoopMonitor struct{}

func NewNoopMonitor() *NoopMonitor {
	return &NoopMonitor{}
}

func (m *NoopMonitor) Start(experimentID string, endpoints []string) {}

func (m *NoopMonitor) Stop() {}

func (m *NoopMonitor) SetIntensity(i int) {}
