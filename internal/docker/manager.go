package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

type Manager struct{}

type dockerStats struct {
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
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

// Docker stats for container

func (m *Manager) GetContainerStats(name string) (float64, float64, float64, string, string, error) {

	out, err := m.run("stats", name, "--no-stream", "--format", "{{json .}}")
	if err != nil {
		return 0, 0, 0, "", "", err
	}

	var stats dockerStats
	err = json.Unmarshal([]byte(out), &stats)
	if err != nil {
		return 0, 0, 0, "", "", err
	}

	cpuStr := strings.TrimSuffix(stats.CPUPerc, "%")
	cpuVal, _ := strconv.ParseFloat(cpuStr, 64)

	memPctStr := strings.TrimSuffix(stats.MemPerc, "%")
	memPctVal, _ := strconv.ParseFloat(memPctStr, 64)

	// MemUsage format: "5.43MiB / 62.8MiB"
	memParts := strings.Split(stats.MemUsage, "/")
	memVal := 0.0

	if len(memParts) > 0 {
		memValStr := strings.TrimSpace(memParts[0])
		memValStr = strings.TrimSuffix(memValStr, "MiB")
		memValStr = strings.TrimSuffix(memValStr, "GiB")

		memVal, _ = strconv.ParseFloat(memValStr, 64)
	}

	return cpuVal, memVal, memPctVal, stats.NetIO, stats.BlockIO, nil
}
