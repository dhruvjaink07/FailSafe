package orchestrator

func (o *Orchestrator) setupDocker(targets []string) {
	for _, t := range targets {
		_ = o.docker.EnsureContainerReady(t, "", "")
	}
}
