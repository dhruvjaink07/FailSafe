package handlers

import (
	"encoding/json"
	"net/http"
)

func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
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
