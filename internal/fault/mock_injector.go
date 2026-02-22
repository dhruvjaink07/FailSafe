package fault

import (
	"fmt"
	"os/exec"
	"strconv"
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
			m.injectCPUStress(config)
		case FaultMemory:
			m.injectMemoryStress(config)
		case FaultKill:
			m.injectKill(config)
		case FaultDelay:
			m.injectNetworkLatency(config)
		case FaultPacketLoss:
			m.injectPacketLoss(config)
		}
	}()

	return nil
}

func (m *MockInjector) injectCPUStress(config FaultConfig) error {

	intensity := config.Intensity
	if intensity <= 0 {
		intensity = 50
	}

	for _, container := range config.Containers {

		cmd := exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			fmt.Sprintf("stress --cpu 1 --timeout %d", config.DurationSeconds),
		)

		_ = cmd.Start()
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)
	return nil
}

func (m *MockInjector) injectMemoryStress(config FaultConfig) error {

	memMB := config.Intensity * 10
	if memMB <= 0 {
		memMB = 100
	}

	for _, container := range config.Containers {

		cmd := exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			fmt.Sprintf("stress --vm 1 --vm-bytes %dM --timeout %d",
				memMB,
				config.DurationSeconds,
			),
		)

		_ = cmd.Start()
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)
	return nil
}

func (m *MockInjector) injectKill(config FaultConfig) error {

	for _, container := range config.Containers {
		_ = m.docker.StopContainer(container)
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	for _, container := range config.Containers {
		_ = m.docker.StartContainer(container)
	}

	return nil
}

func (m *MockInjector) injectNetworkLatency(config FaultConfig) error {

	delayMs := config.Intensity * 10
	if delayMs <= 0 {
		delayMs = 200
	}

	for _, container := range config.Containers {

		// Add delay
		exec.Command(
			"docker", "exec", container,
			"tc", "qdisc", "add", "dev", "eth0",
			"root", "netem", "delay",
			strconv.Itoa(delayMs)+"ms",
		).Run()
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	// Remove delay
	for _, container := range config.Containers {
		exec.Command(
			"docker", "exec", container,
			"tc", "qdisc", "del", "dev", "eth0", "root",
		).Run()
	}

	return nil
}

func (m *MockInjector) injectPacketLoss(config FaultConfig) error {

	lossPercent := config.Intensity
	if lossPercent <= 0 {
		lossPercent = 20
	}

	for _, container := range config.Containers {

		exec.Command(
			"docker", "exec", container,
			"tc", "qdisc", "add", "dev", "eth0",
			"root", "netem", "loss",
			strconv.Itoa(lossPercent)+"%",
		).Run()
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	for _, container := range config.Containers {
		exec.Command(
			"docker", "exec", container,
			"tc", "qdisc", "del", "dev", "eth0", "root",
		).Run()
	}

	return nil
}
