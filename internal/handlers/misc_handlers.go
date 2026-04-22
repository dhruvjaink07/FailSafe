package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-api-key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		// Basic health
		status := map[string]interface{}{"server": "ok"}

		// If a python grpc address is configured, check its health too.
		addr := "localhost:50051"
		if v := os.Getenv("PYTHON_GRPC_ADDR"); v != "" {
			addr = v
		}
		if err := checkPythonHealth(r.Context(), addr, 1*time.Second); err != nil {
			status["python"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			status["python"] = map[string]interface{}{"status": "ok"}
			w.WriteHeader(http.StatusOK)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}
}

func ScenarioPresetsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := map[string]interface{}{
			"available_presets": []string{
				"login_flow_network_failure",
				"background_sync_interruption",
				"permission_revoked_mid_usage",
			},
			"fault_types": []string{
				"kill_app",
				"kill_repeated",
				"network_disable",
				"network_enable",
				"network_flaky",
				"network_latency",
				"network_packet_loss",
				"revoke_camera",
				"revoke_storage",
				"revoke_location",
				"background_app",
				"foreground_app",
				"clear_data",
			},
			"trigger_types": []string{
				"request",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

func SystemMetricsHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		metrics, err := orch.GetLatestSystemMetrics()
		if err != nil {
			http.Error(w, "failed to fetch system metrics: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(metrics)
	}
}

func ExperimentsListHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		experiments, err := orch.ListExperiments()
		if err != nil {
			http.Error(w, "failed to fetch experiments: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(experiments)
	}
}
