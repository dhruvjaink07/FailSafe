package fault

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
)

type DockerInjector struct {
	docker *docker.Manager
}

func NewDockerInjector(dm *docker.Manager) *DockerInjector {
	return &DockerInjector{
		docker: dm,
	}
}

func (m *DockerInjector) Inject(config FaultConfig) error {

	fmt.Printf("Injecting %s at intensity %d on %v\n",
		config.Type, config.Intensity, config.Targets)

	go func() {
		switch config.Type {
		case FaultCPU:
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

// ---------------- CPU ----------------

func (m *DockerInjector) injectCPUStress(config FaultConfig) error {

	// ensure stress exists
	for _, container := range config.Targets {
		exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			"which stress || (apt update && apt install -y stress)",
		).Run()
	}

	for _, container := range config.Targets {

		cmd := exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			fmt.Sprintf(
				"stress --cpu %d --timeout %d",
				max(1, config.Intensity/20),
				config.DurationSeconds,
			),
		)

		if err := cmd.Start(); err != nil {
			fmt.Println("cpu inject error:", err)
		}
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)
	return nil
}

// ---------------- MEMORY ----------------

func (m *DockerInjector) injectMemoryStress(config FaultConfig) error {

	memMB := config.Intensity * 50
	if memMB > 1024 {
		memMB = 1024
	}
	if memMB <= 0 {
		memMB = 100
	}

	// ensure stress exists
	for _, container := range config.Targets {
		exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			"which stress || (apt update && apt install -y stress)",
		).Run()
	}

	for _, container := range config.Targets {

		cmd := exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			fmt.Sprintf(
				"stress --vm %d --vm-bytes %dM --timeout %d",
				max(1, config.Intensity/25),
				memMB,
				config.DurationSeconds,
			),
		)

		if err := cmd.Start(); err != nil {
			fmt.Println("memory inject error:", err)
		}
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	// cleanup
	for _, container := range config.Targets {
		exec.Command(
			"docker", "exec", container,
			"pkill", "-f", "stress",
		).Run()
	}

	return nil
}

// ---------------- KILL ----------------

func (m *DockerInjector) injectKill(config FaultConfig) error {

	interval := 2 * time.Second
	end := time.Now().Add(time.Duration(config.DurationSeconds) * time.Second)

	for time.Now().Before(end) {

		// stagger stop
		for i, container := range config.Targets {
			time.Sleep(time.Duration(i) * 300 * time.Millisecond)
			_ = m.docker.StopContainer(container)
		}

		time.Sleep(interval)

		// stagger start
		for i, container := range config.Targets {
			time.Sleep(time.Duration(i) * 300 * time.Millisecond)
			_ = m.docker.StartContainer(container)
		}

		time.Sleep(interval)
	}

	return nil
}

// ---------------- NETWORK DELAY ----------------

func (m *DockerInjector) injectNetworkLatency(config FaultConfig) error {

	// ensure tc exists
	for _, container := range config.Targets {
		exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			"which tc || (apt update && apt install -y iproute2)",
		).Run()
	}

	delayMs := int(math.Pow(float64(config.Intensity), 1.3)) * 10
	if delayMs <= 0 {
		delayMs = 500
	}

	jitter := delayMs / 2
	loss := config.Intensity / 5
	if loss > 25 {
		loss = 25
	}

	// small delay before applying → propagation wave
	time.Sleep(2 * time.Second)

	for _, container := range config.Targets {

		// clear existing
		exec.Command(
			"docker", "exec", container,
			"sh", "-c", "tc qdisc del dev eth0 root || true",
		).Run()

		err := exec.Command(
			"docker", "exec", container,
			"tc", "qdisc", "replace", "dev", "eth0",
			"root", "netem",
			"delay", fmt.Sprintf("%dms %dms", delayMs, jitter),
			"loss", strconv.Itoa(loss)+"%",
		).Run()

		if err != nil {
			fmt.Println("network delay error:", err)
		}
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	for _, container := range config.Targets {
		exec.Command(
			"docker", "exec", container,
			"sh", "-c", "tc qdisc del dev eth0 root || true",
		).Run()
	}

	return nil
}

// ---------------- PACKET LOSS ----------------

func (m *DockerInjector) injectPacketLoss(config FaultConfig) error {

	// ensure tc exists
	for _, container := range config.Targets {
		exec.Command(
			"docker", "exec", container,
			"sh", "-c",
			"which tc || (apt update && apt install -y iproute2)",
		).Run()
	}

	loss := config.Intensity
	if loss <= 0 {
		loss = 20
	}
	if loss > 50 {
		loss = 50
	}

	delay := config.Intensity * 20
	if delay == 0 {
		delay = 200
	}

	for _, container := range config.Targets {

		exec.Command(
			"docker", "exec", container,
			"sh", "-c", "tc qdisc del dev eth0 root || true",
		).Run()

		err := exec.Command(
			"docker", "exec", container,
			"tc", "qdisc", "replace", "dev", "eth0",
			"root", "netem",
			"loss", strconv.Itoa(loss)+"%",
			"delay", fmt.Sprintf("%dms %dms", delay, delay/2),
		).Run()

		if err != nil {
			fmt.Println("packet loss error:", err)
		}
	}

	time.Sleep(time.Duration(config.DurationSeconds) * time.Second)

	for _, container := range config.Targets {
		exec.Command(
			"docker", "exec", container,
			"sh", "-c", "tc qdisc del dev eth0 root || true",
		).Run()
	}

	return nil
}

// ---------------- UTILS ----------------

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
