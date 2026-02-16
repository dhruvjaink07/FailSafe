package docker

import (
	"testing"
	"time"
)

func TestContainerLifeCycle(t *testing.T) {

	manager := NewManager()

	containerName := "failsafe-test-container"
	imageName := "nginx:latest"
	portMapping := "8090:80"

	// Ensure clean start
	_ = manager.StopContainer(containerName)
	_ = manager.RemoveContainer(containerName)

	// Ensure Container ready
	err := manager.EnsureContainerReady(containerName, imageName, portMapping)
	if err != nil {
		t.Fatalf("failed to ensure container ready: %v", err)
	}

	// Check exists
	exists, err := manager.ContainerExists(containerName)
	if err != nil {
		t.Fatalf("error checking container existence: %v", err)
	}
	if !exists {
		t.Fatalf("container  should exist")
	}

	// Check running
	running, err := manager.ContainerRunning(containerName)
	if err != nil {
		t.Fatalf("error checking container running state: %v", err)
	}
	if !running {
		t.Fatalf("container should be running")
	}

	// Give container a second to stabilize
	time.Sleep(2 * time.Second)

	// Stop Container
	err = manager.StopContainer(containerName)
	if err != nil {
		t.Fatalf("failed to stop container: %v", err)
	}

	running, _ = manager.ContainerRunning(containerName)
	if running {
		t.Fatalf("container should not be running after stop")
	}

	// sRemove container
	err = manager.RemoveContainer(containerName)
	if err != nil {
		t.Fatalf("failed to remove container: %v", err)
	}

	exists, _ = manager.ContainerExists(containerName)
	if exists {
		t.Fatalf("container should not exist after removal")
	}
}
