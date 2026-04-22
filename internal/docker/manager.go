package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Manager struct{}

type dockerStats struct {
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
}
type ContainerStats struct {
	CPUPercent    float64
	MemoryMB      float64
	MemoryPercent float64
	NetworkIO     string
	BlockIO       string
}

type ContainerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Ports   string `json:"ports"`
	Running bool   `json:"running"`
}

type EngineStartResult struct {
	OS             string `json:"os"`
	AlreadyRunning bool   `json:"already_running"`
	DesktopStarted bool   `json:"desktop_started"`
	EngineReady    bool   `json:"engine_ready"`
	Message        string `json:"message"`
}

func NewManager() *Manager {
	return &Manager{}
}

/*
---------------------------------------
UTILITY: run docker command
---------------------------------------
*/

func (m *Manager) run(args ...string) (string, error) {

	cmd := exec.Command("docker", args...)

	var out bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", errors.New(stderr.String())
	}

	return strings.TrimSpace(out.String()), nil
}

/*
---------------------------------------
IMAGE MANAGEMENT
---------------------------------------
*/

// Check if image exists locally
func (m *Manager) ImageExists(image string) (bool, error) {
	out, err := m.run("images", "-q", image)
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// Pull image if missing
func (m *Manager) EnsureImage(image string) error {

	exists, err := m.ImageExists(image)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	_, err = m.run("pull", image)
	return err
}

/*
---------------------------------------
CONTAINER MANAGEMENT
---------------------------------------
*/
// Check is container exists
func (m *Manager) ContainerExists(name string) (bool, error) {

	out, err := m.run("ps", "-a", "--filter", "name="+name, "--format", "{{.Names}}")
	if err != nil {
		return false, err
	}

	return out == name, err
}

// Check if container is running
func (m *Manager) ContainerRunning(name string) (bool, error) {

	out, err := m.run("inspect", "-f", "{{.State.Running}}", name)
	if err != nil {
		return false, err
	}

	return out == "true", nil
}

// Create Container (if not exists)
func (m *Manager) CreateContainer(name, image string, portMapping string) error {

	exists, err := m.ContainerExists(name)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	args := []string{
		"run",
		"-d",
		"--name", name,
	}

	if portMapping != "" {
		args = append(args, "-p", portMapping)
	}

	args = append(args, image)

	_, err = m.run(args...)
	return err
}

// Start Container
func (m *Manager) StartContainer(name string) error {

	running, _ := m.ContainerRunning(name)
	if running {
		return nil
	}

	_, err := m.run("start", name)
	return err
}

// Stop Container
func (m *Manager) StopContainer(name string) error {

	running, _ := m.ContainerRunning(name)

	if !running {
		return nil
	}

	_, err := m.run("stop", name)
	return err
}

// Remove Container
func (m *Manager) RemoveContainer(name string) error {

	exists, err := m.ContainerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	_, err = m.run("rm", name)
	return err
}

/*
---------------------------------------
ENSURE CONTAINER READY
---------------------------------------
*/

func (m *Manager) EnsureContainerReady(name, image, portMapping string) error {

	err := m.EnsureImage(image)
	if err != nil {
		return err
	}

	err = m.CreateContainer(name, image, portMapping)
	if err != nil {
		return err
	}

	return m.StartContainer(name)
}

// To run any docker exec command inside container
func (m *Manager) Exec(container string, args ...string) error {
	cmdArgs := append([]string{"exec", container}, args...)

	_, err := m.run(cmdArgs...)
	return err
}

// Docker stats for container
func (m *Manager) GetContainerStats(name string) (*ContainerStats, error) {

	out, err := m.run("stats", name, "--no-stream", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}

	var stats dockerStats
	err = json.Unmarshal([]byte(out), &stats)
	if err != nil {
		return nil, err
	}

	cpuStr := strings.TrimSuffix(stats.CPUPerc, "%")
	cpuVal, _ := strconv.ParseFloat(cpuStr, 64)

	memPctStr := strings.TrimSuffix(stats.MemPerc, "%")
	memPctVal, _ := strconv.ParseFloat(memPctStr, 64)

	// MemUsage example: "5.43MiB / 62.8MiB"
	memParts := strings.Split(stats.MemUsage, "/")
	memVal := 0.0

	if len(memParts) > 0 {
		memValStr := strings.TrimSpace(memParts[0])

		if strings.Contains(memValStr, "MiB") {
			memValStr = strings.TrimSuffix(memValStr, "MiB")
			memVal, _ = strconv.ParseFloat(strings.TrimSpace(memValStr), 64)
		}

		if strings.Contains(memValStr, "GiB") {
			memValStr = strings.TrimSuffix(memValStr, "GiB")
			val, _ := strconv.ParseFloat(strings.TrimSpace(memValStr), 64)
			memVal = val * 1024
		}
	}

	return &ContainerStats{
		CPUPercent:    cpuVal,
		MemoryMB:      memVal,
		MemoryPercent: memPctVal,
		NetworkIO:     stats.NetIO,
		BlockIO:       stats.BlockIO,
	}, nil
}

// GetContainerLogs returns the logs for the given container name. If tail > 0,
// it will include only the last `tail` lines.
func (m *Manager) GetContainerLogs(name string, tail int) (string, error) {
	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprint(tail))
	}
	args = append(args, name)

	out, err := m.run(args...)
	if err != nil {
		return "", err
	}

	return out, nil
}

// StreamContainerLogs starts `docker logs` for the container and returns an
// io.ReadCloser to stream output, and the underlying *exec.Cmd so caller can
// stop it. If follow is true, uses `--follow`.
func (m *Manager) StreamContainerLogs(name string, tail int, follow bool) (io.ReadCloser, *exec.Cmd, error) {
	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprint(tail))
	}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, name)

	cmd := exec.Command("docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	_, err = cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	// Merge stderr into stdout stream by creating a reader that reads from stdout first then stderr
	// Simpler: return stdout only; stderr will be lost. For now, return stdout.
	return stdout, cmd, nil
}

// ListContainers returns all containers visible to docker ps -a.
func (m *Manager) ListContainers() ([]ContainerInfo, error) {
	out, err := m.run("ps", "-a", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}}\t{{.Ports}}")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(out) == "" {
		return []ContainerInfo{}, nil
	}

	lines := strings.Split(out, "\n")
	containers := make([]ContainerInfo, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 6)
		if len(parts) < 6 {
			continue
		}

		state := strings.TrimSpace(parts[3])
		containers = append(containers, ContainerInfo{
			ID:      strings.TrimSpace(parts[0]),
			Name:    strings.TrimSpace(parts[1]),
			Image:   strings.TrimSpace(parts[2]),
			State:   state,
			Status:  strings.TrimSpace(parts[4]),
			Ports:   strings.TrimSpace(parts[5]),
			Running: strings.EqualFold(state, "running"),
		})
	}

	return containers, nil
}

func (m *Manager) dockerAvailable() bool {
	_, err := m.run("version", "--format", "{{.Server.Version}}")
	return err == nil
}

// EnsureDockerEngine tries to start Docker Desktop on Windows/macOS and waits until engine is reachable.
func (m *Manager) EnsureDockerEngine() (EngineStartResult, error) {
	res := EngineStartResult{OS: runtime.GOOS}

	if m.dockerAvailable() {
		res.AlreadyRunning = true
		res.EngineReady = true
		res.Message = "docker engine already running"
		return res, nil
	}

	if err := m.startEngineHost(); err != nil {
		res.Message = err.Error()
		return res, err
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		res.DesktopStarted = true
	}

	timeout := time.Now().Add(90 * time.Second)
	for time.Now().Before(timeout) {
		if m.dockerAvailable() {
			res.EngineReady = true
			res.Message = "docker desktop started and engine is reachable"
			return res, nil
		}
		time.Sleep(2 * time.Second)
	}

	res.Message = "docker start attempted but engine not ready within 90 seconds"
	return res, errors.New(res.Message)
}

func (m *Manager) startEngineHost() error {
	switch runtime.GOOS {
	case "windows":
		candidates := []string{
			`C:\Program Files\Docker\Docker\Docker Desktop.exe`,
			`C:\Program Files\Docker\Docker\com.docker.desktop.exe`,
		}

		for _, exe := range candidates {
			if err := exec.Command(exe).Start(); err == nil {
				return nil
			}
		}
		return errors.New("docker desktop executable not found on Windows")
	case "darwin":
		if err := exec.Command("open", "-a", "Docker").Start(); err != nil {
			return fmt.Errorf("failed to launch Docker on macOS: %w", err)
		}
		return nil
	case "linux":
		// Try systemd first (most common Linux setup).
		if err := exec.Command("systemctl", "start", "docker").Run(); err == nil {
			return nil
		}
		// Fallback for non-systemd distros.
		if err := exec.Command("service", "docker", "start").Run(); err == nil {
			return nil
		}
		return errors.New("failed to start Docker engine on Linux (tried: systemctl start docker, service docker start). Ensure Docker is installed and the process has sufficient privileges")
	default:
		return fmt.Errorf("unsupported OS for docker engine start: %s", runtime.GOOS)
	}
}
