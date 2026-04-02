package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func ExperimentFrontendStartHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req StartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Println("JSON decode error:", err)
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := applyScenarioPreset(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.TargetType != string(models.TargetFrontend) {
			http.Error(w, "target_type must be frontend for frontend endpoint", http.StatusBadRequest)
			return
		}
		if req.FaultType == "" {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}
		if req.FrontendRun == nil || strings.TrimSpace(req.FrontendRun.BaseURL) == "" {
			http.Error(w, "frontendRun.baseUrl is required for target_type=frontend", http.StatusBadRequest)
			return
		}

		exp, err := orch.StartExperiment(
			req.FaultType,
			req.Targets,
			req.TargetType,
			req.ObservedEndpoints,
			req.ObservationType,
			req.Duration,
			req.Adaptive,
			req.StepIntensity,
			req.MaxIntensity,
			req.DependencyGraph,
			req.TargetEndpointMap,
			req.Scenario,
			req.Expected,
			nil,
			nil,
			req.FrontendRun.toModel(),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(exp)
	}
}

func ExperimentFrontendStatusHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id, err := requireExperimentID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := ensureTargetType(orch, id, models.TargetFrontend); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		status, err := orch.GetExperiment(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	}
}

func ExperimentFrontendStopHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id, err := requireExperimentID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := ensureTargetType(orch, id, models.TargetFrontend); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := orch.StopExperiment(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("experiment stopped"))
	}
}

func ExperimentFrontendMetricsHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id, err := requireExperimentID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		data, err := orch.GetFrontendMetrics(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(data)
	}
}
