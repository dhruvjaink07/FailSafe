package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func ExperimentBackendStartHandler(orch ExperimentService) http.HandlerFunc {
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

		if req.TargetType != string(models.TargetDocker) {
			http.Error(w, "target_type must be docker for backend endpoint", http.StatusBadRequest)
			return
		}
		if req.FaultType == "" || len(req.Targets) == 0 {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}

		var apiCtx *models.APIContext
		if ctx, ok := APIContextFromRequest(r); ok {
			apiCtx = &ctx
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
			apiCtx,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(exp)
	}
}

func ExperimentBackendStatusHandler(orch ExperimentService) http.HandlerFunc {
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
		if err := ensureTargetType(orch, id, models.TargetDocker); err != nil {
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

func ExperimentBackendStopHandler(orch ExperimentService) http.HandlerFunc {
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
		if err := ensureTargetType(orch, id, models.TargetDocker); err != nil {
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

func ExperimentBackendMetricsHandler(orch ExperimentService) http.HandlerFunc {
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

		data, err := orch.GetBackendMetrics(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(data)
	}
}
