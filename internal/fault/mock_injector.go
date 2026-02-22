package fault

import (
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
)

type MockInjector struct {
	docker *docker.Manager
}

func NewMockInjector(dm *docker.Manager) *MockInjector {
	return &MockInjector{
		docker: dm,
	}
}

func (m *MockInjector) Inject(config FaultConfig) error {
	go func() {
		switch config.Type {
		case FaultCPU:
			// Simulate CPU stress by running a busy loop for the duration
			m.injectCPU(config)
		case FaultMemory:
			m.injectMemory(config)
		case FaultKill:
			m.injectKill(config)
		case FaultDelay:
			m.injectNetworkDelay(config)
		}
	}()

	return nil
}

func (m *MockInjector) injectCPU(config FaultConfig) {
	for _, container := range config.Containers {
		// Add CPU stress process inside container
		m.docker.Exec(container, "sh", "-c", "yes > /dev/null &")
	}
	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)
	for _, container := range config.Containers {
		// kill stress process
		m.docker.Exec(container, "pkill", "yes")
	}
}

func (m *MockInjector) injectMemory(config FaultConfig) {
	for _, container := range config.Containers {
		// Add CPU stress process inside container
		m.docker.Exec(container, "sh", "-c", "yes > /dev/null &")
	}
	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	for _, container := range config.Containers {
		m.docker.Exec(container, "pkill", "tail")
	}
}

func (m *MockInjector) injectKill(config FaultConfig) {

	// Stop all target containers
	for _, container := range config.Containers {
		_ = m.docker.StopContainer(container)
	}

	// Hold fault duration
	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	// Restart all containers
	for _, container := range config.Containers {
		_ = m.docker.StartContainer(container)
	}
}

func (m *MockInjector) injectNetworkDelay(config FaultConfig) {
	for _, container := range config.Containers {
		// Simulate network delay usinf tc
		m.docker.Exec(container, "sh", "-c", "tc qdisc add dev eth0 root netem delay 500ms")
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)
	for _, container := range config.Containers {
		m.docker.Exec(container, "sh", "-c", "tc qdisc del dev eth0 root netem")
	}
}
