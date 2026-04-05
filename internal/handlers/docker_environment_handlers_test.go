package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhruvjaink07/failsafe/internal/docker"
)

type fakeDockerEnvironmentService struct {
	containers []docker.ContainerInfo
	startErr   error
	started    []string
}

func (f *fakeDockerEnvironmentService) ListContainers() ([]docker.ContainerInfo, error) {
	return f.containers, nil
}

func (f *fakeDockerEnvironmentService) StartContainer(name string) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = append(f.started, name)
	return nil
}

func TestDockerContainersListHandler(t *testing.T) {
	fake := &fakeDockerEnvironmentService{
		containers: []docker.ContainerInfo{
			{Name: "z-stopped", Running: false, State: "exited"},
			{Name: "a-running", Running: true, State: "running"},
		},
	}

	h := DockerContainersListHandler(fake)
	req := httptest.NewRequest(http.MethodGet, "/environment/containers", nil)
	res := httptest.NewRecorder()
	h(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	containers, ok := body["containers"].([]interface{})
	if !ok || len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %v", body["containers"])
	}

	first, _ := containers[0].(map[string]interface{})
	if first["name"] != "a-running" {
		t.Fatalf("expected running container first, got %+v", first)
	}
}

func TestDockerContainerStartHandlerQuery(t *testing.T) {
	fake := &fakeDockerEnvironmentService{}
	h := DockerContainerStartHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/environment/containers/start?name=failsafe-postgres", nil)
	res := httptest.NewRecorder()
	h(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	if len(fake.started) != 1 || fake.started[0] != "failsafe-postgres" {
		t.Fatalf("unexpected started containers: %+v", fake.started)
	}
}

func TestDockerContainerStartHandlerBody(t *testing.T) {
	fake := &fakeDockerEnvironmentService{}
	h := DockerContainerStartHandler(fake)

	body := bytes.NewBufferString(`{"name":"failsafe-backend"}`)
	req := httptest.NewRequest(http.MethodPost, "/environment/containers/start", body)
	res := httptest.NewRecorder()
	h(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	if len(fake.started) != 1 || fake.started[0] != "failsafe-backend" {
		t.Fatalf("unexpected started containers: %+v", fake.started)
	}
}

func TestDockerContainerStartHandlerValidation(t *testing.T) {
	fake := &fakeDockerEnvironmentService{}
	h := DockerContainerStartHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/environment/containers/start", bytes.NewBufferString("{}"))
	res := httptest.NewRecorder()
	h(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestDockerContainerStartHandlerServiceError(t *testing.T) {
	fake := &fakeDockerEnvironmentService{startErr: errors.New("no such container")}
	h := DockerContainerStartHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/environment/containers/start?name=missing", nil)
	res := httptest.NewRecorder()
	h(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", res.Code, res.Body.String())
	}
}
