package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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

		apiCtx, _ := APIContextFromRequest(r)

		limit := parseBoundedInt(r.URL.Query().Get("limit"), defaultHistoryLimit, 1, maxHistoryLimit)
		offset := parseBoundedInt(r.URL.Query().Get("offset"), 0, 0, 1000000)

		items, err := orch.GetExperimentHistory(apiCtx.KeyID, limit, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		total, err := orch.GetExperimentHistoryCount(apiCtx.KeyID)
		if err != nil || total <= 0 {
			// Fallback: if count query failed or returned 0, use returned items length
			total = len(items)
		}

		// Compute paging metadata
		pageSize := limit
		totalPages := 0
		if pageSize > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}
		page := 0
		if pageSize > 0 {
			page = (offset / pageSize) + 1
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items":       items,
			"count":       len(items),
			"total_count": total,
			"limit":       limit,
			"offset":      offset,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": totalPages,
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

		apiCtx, _ := APIContextFromRequest(r)
		tail := parseBoundedInt(r.URL.Query().Get("tail"), 0, 0, 10000)

		format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
		follow := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("follow"))) == "true"

		// If follow requested, use streaming path on orchestrator
		if follow {
			if err := orch.StreamBackendLogs(apiCtx.KeyID, experimentID, tail, w, format, true); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}

		logs, err := orch.GetBackendLogs(apiCtx.KeyID, experimentID, tail)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Support optional JSON format: ?format=json -> { "lines": [...], "count": N }
		if format == "json" {
			lines := strings.Split(logs, "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
				lines = lines[:len(lines)-1]
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"lines": lines, "count": len(lines)})
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

		apiCtx, _ := APIContextFromRequest(r)

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

// ExperimentHistoryCountHandler returns only the total count of history records.
func ExperimentHistoryCountHandler(orch ExperimentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		apiCtx, _ := APIContextFromRequest(r)

		total, err := orch.GetExperimentHistoryCount(apiCtx.KeyID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": total,
		})
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
