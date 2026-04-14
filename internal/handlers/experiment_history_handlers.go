package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

func ExperimentHistoryHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		apiCtx, ok := APIContextFromRequest(r)
		if !ok || apiCtx.KeyID == "" {
			http.Error(w, "missing api context", http.StatusUnauthorized)
			return
		}

		limit := parseBoundedInt(r.URL.Query().Get("limit"), defaultHistoryLimit, 1, maxHistoryLimit)
		offset := parseBoundedInt(r.URL.Query().Get("offset"), 0, 0, 1000000)

		items, err := orch.GetExperimentHistory(apiCtx.KeyID, limit, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items":  items,
			"count":  len(items),
			"limit":  limit,
			"offset": offset,
		})
	}
}

func ExperimentBackendLogsHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		experimentID, err := requireExperimentID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		apiCtx, ok := APIContextFromRequest(r)
		if !ok || apiCtx.KeyID == "" {
			http.Error(w, "missing api context", http.StatusUnauthorized)
			return
		}

		tail := parseBoundedInt(r.URL.Query().Get("tail"), 0, 0, 10000)

		logs, err := orch.GetBackendLogs(apiCtx.KeyID, experimentID, tail)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(logs))
	}
}

func ExperimentHistoryDetailHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		apiCtx, ok := APIContextFromRequest(r)
		if !ok || apiCtx.KeyID == "" {
			http.Error(w, "missing api context", http.StatusUnauthorized)
			return
		}

		experimentID, err := requireExperimentID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		item, err := orch.GetExperimentHistoryDetail(apiCtx.KeyID, experimentID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if item == nil {
			http.Error(w, "experiment not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(item)
	}
}

func parseBoundedInt(raw string, fallback, min, max int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
