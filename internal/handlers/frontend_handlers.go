package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
)

func FrontendMetricsHandler(o *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

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
