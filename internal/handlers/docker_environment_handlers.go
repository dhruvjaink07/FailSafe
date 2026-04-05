package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/docker"
)

type DockerEnvironmentService interface {
	ListContainers() ([]docker.ContainerInfo, error)
	StartContainer(name string) error
}

type dockerStartRequest struct {
	Name string `json:"name"`
}

func DockerContainersListHandler(service DockerEnvironmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		containers, err := service.ListContainers()
		if err != nil {
			http.Error(w, "failed to list containers: "+err.Error(), http.StatusInternalServerError)
			return
		}

		sort.Slice(containers, func(i, j int) bool {
			if containers[i].Running != containers[j].Running {
				return containers[i].Running
			}
			return containers[i].Name < containers[j].Name
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"count":      len(containers),
			"containers": containers,
		})
	}
}

func DockerContainerStartHandler(service DockerEnvironmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			var req dockerStartRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				name = strings.TrimSpace(req.Name)
			}
		}

		if name == "" {
			http.Error(w, "container name is required (query: ?name=... or body: {\"name\":\"...\"})", http.StatusBadRequest)
			return
		}

		if err := service.StartContainer(name); err != nil {
			http.Error(w, "failed to start container: "+err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name":       name,
			"status":     "started_or_already_running",
			"started_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
