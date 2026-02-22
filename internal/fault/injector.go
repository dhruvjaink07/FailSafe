package fault

type Injector struct {
}

func (i *Injector) Inject(config FaultConfig) error {
	return nil
}
