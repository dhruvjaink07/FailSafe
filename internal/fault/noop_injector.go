package fault

type NoopInjector struct{}

func NewNoopInjector() *NoopInjector {
	return &NoopInjector{}
}

func (n *NoopInjector) Inject(config FaultConfig) error {
	return nil
}
