package fault

type Injector interface {
	Inject(config FaultConfig) error
}
