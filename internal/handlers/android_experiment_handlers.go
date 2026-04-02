package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
)

func ExperimentAndroidStartHandler(orch ExperimentService) http.HandlerFunc {
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

		if req.TargetType != string(models.TargetAndroid) {
			http.Error(w, "target_type must be android for android endpoint", http.StatusBadRequest)
			return
		}
		if req.FaultType == "" || len(req.Targets) == 0 {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}

		var appCfg *orchestrator.AndroidAppConfig
		if strings.TrimSpace(req.APK) != "" {
			apkMeta, ok := getUploadedAPK(req.APK)
			if !ok {
				http.Error(w, "invalid apk reference: upload id not found", http.StatusBadRequest)
				return
			}
			appCfg = &orchestrator.AndroidAppConfig{
				APKPath:  apkMeta.Path,
				Package:  apkMeta.Package,
				Activity: apkMeta.Activity,
			}
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
			req.AndroidRun.toOptions(),
			appCfg,
			nil,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(exp)
	}
}

func ExperimentAndroidStatusHandler(orch ExperimentService) http.HandlerFunc {
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
		if err := ensureTargetType(orch, id, models.TargetAndroid); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		status, err := orch.GetAndroidStatus(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}
}

func ExperimentAndroidStopHandler(orch ExperimentService) http.HandlerFunc {
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
		if err := ensureTargetType(orch, id, models.TargetAndroid); err != nil {
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

func ExperimentAndroidMetricsHandler(orch ExperimentService) http.HandlerFunc {
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

		data, err := orch.GetAndroidMetrics(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(data)
	}
}
