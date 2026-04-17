package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

		// Persist raw payload for debugging/inspection
		go func(b models.FrontendMetricsBatch) {
			defer func() { _ = recover() }()
			outDir := filepath.Join("experiments", "results")
			_ = os.MkdirAll(outDir, 0o755)
			var id string
			if len(b.Metrics) > 0 {
				id = b.Metrics[0].ExperimentID
			}
			if id == "" {
				id = "unknown"
			}
			fname := filepath.Join(outDir, id+"-controller-ingest-"+time.Now().Format("20060102-150405")+".json")
			if data, err := json.MarshalIndent(b, "", "  "); err == nil {
				_ = os.WriteFile(fname, data, 0o644)
			}
		}(batch)

		o.AddFrontendMetrics(batch.Metrics)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
