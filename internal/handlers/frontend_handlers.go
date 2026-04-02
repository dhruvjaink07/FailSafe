package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func FrontendMetricsHandler(o ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS for browser requests
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var batch models.FrontendMetricsBatch

		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		o.AddFrontendMetrics(batch.Metrics)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
