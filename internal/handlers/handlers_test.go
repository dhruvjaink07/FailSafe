package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
)

type FakeOrch struct {
	items []map[string]interface{}
	logs  string
}

func (f *FakeOrch) StartExperiment(faultType string, targets []string, targetType string, observedEndpoints []string, observationType string, duration int, adaptive bool, stepIntensity int, maxIntensity int, deps models.DependencyGraph, targetMap map[string][]string, scheduledFaults []models.ScheduledFault, expected models.ExpectedState, androidOptions *orchestrator.AndroidRunOptions, androidApp *orchestrator.AndroidAppConfig, frontendRun *models.FrontendRunConfig, apiCtx *models.APIContext) (*models.Experiment, error) {
	return nil, nil
}
func (f *FakeOrch) GetExperiment(id string) (map[string]interface{}, error)    { return nil, nil }
func (f *FakeOrch) StopExperiment(id string) error                             { return nil }
func (f *FakeOrch) GetBackendMetrics(id string) (interface{}, error)           { return nil, nil }
func (f *FakeOrch) GetAndroidMetrics(id string) (interface{}, error)           { return nil, nil }
func (f *FakeOrch) GetAndroidStatus(id string) (map[string]interface{}, error) { return nil, nil }
func (f *FakeOrch) GetExperimentTargetType(id string) (string, error)          { return "", nil }
func (f *FakeOrch) GetFrontendMetrics(id string) (interface{}, error)          { return nil, nil }
func (f *FakeOrch) GetFrontendFaultCommand(id string) (map[string]interface{}, error) {
	return nil, nil
}

func (f *FakeOrch) GetExperimentHistory(apiKeyID string, limit int, offset int) ([]map[string]interface{}, error) {
	// emulate DB-backed items with offset/limit
	total := len(f.items)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []map[string]interface{}{}, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return f.items[offset:end], nil
}

func (f *FakeOrch) GetExperimentHistoryCount(apiKeyID string) (int, error) {
	return len(f.items), nil
}

func (f *FakeOrch) GetExperimentHistoryDetail(apiKeyID string, experimentID string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *FakeOrch) GetBackendLogs(apiKeyID string, experimentID string, tail int) (string, error) {
	return f.logs, nil
}
func (f *FakeOrch) StreamBackendLogs(apiKeyID string, experimentID string, tail int, w http.ResponseWriter, format string, follow bool) error {
	// For tests, just write logs to writer and return
	if format == "json" {
		lines := strings.Split(f.logs, "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		return json.NewEncoder(w).Encode(map[string]interface{}{"lines": lines, "count": len(lines)})
	}
	_, err := w.Write([]byte(f.logs))
	return err
}
func (f *FakeOrch) AddFrontendMetrics(data []models.FrontendMetrics) {}

func (f *FakeOrch) GetLatestSystemMetrics() (map[string]interface{}, error) {
	return map[string]interface{}{
		"backend":  map[string]interface{}{},
		"android":  map[string]interface{}{},
		"frontend": map[string]interface{}{},
	}, nil
}

func (f *FakeOrch) ListExperiments() ([]*models.Experiment, error) {
	return []*models.Experiment{}, nil
}

func makeFakeItems(n int) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, map[string]interface{}{"id": strconv.Itoa(i), "value": i})
	}
	return out
}

func TestExperimentHistoryHandler(t *testing.T) {
	fake := &FakeOrch{items: makeFakeItems(7)}
	h := ExperimentHistoryHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/experiments/history?limit=2&offset=0", nil)
	// inject permissive API context
	ctx := req.Context()
	ctx = contextWithAPI(ctx, models.APIContext{KeyID: ""})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if tc, ok := body["total_count"].(float64); !ok || int(tc) != 7 {
		t.Fatalf("expected total_count=7, got %#v", body["total_count"])
	}
	if cnt, ok := body["count"].(float64); !ok || int(cnt) != 2 {
		t.Fatalf("expected count=2, got %#v", body["count"])
	}
}

func TestExperimentHistoryCountHandler(t *testing.T) {
	fake := &FakeOrch{items: makeFakeItems(7)}
	h := ExperimentHistoryCountHandler(fake)
	req := httptest.NewRequest(http.MethodGet, "/experiments/history/count", nil)
	ctx := req.Context()
	ctx = contextWithAPI(ctx, models.APIContext{KeyID: ""})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if tc, ok := body["total_count"].(float64); !ok || int(tc) != 7 {
		t.Fatalf("expected total_count=7, got %#v", body["total_count"])
	}
}

func TestExperimentBackendLogsHandler(t *testing.T) {
	fake := &FakeOrch{logs: "line1\nline2\n"}
	h := ExperimentBackendLogsHandler(fake)

	u := "/experiments/backend/logs"
	q := url.Values{}
	q.Set("id", "abc")
	q.Set("tail", "10")
	req := httptest.NewRequest(http.MethodGet, u+"?"+q.Encode(), nil)
	ctx := req.Context()
	ctx = contextWithAPI(ctx, models.APIContext{KeyID: ""})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(fake.logs) {
		t.Fatalf("unexpected logs: %q", rr.Body.String())
	}

	// test JSON format
	req2 := httptest.NewRequest(http.MethodGet, u+"?"+q.Encode()+"&format=json", nil)
	req2 = req2.WithContext(req2.Context())
	req2 = req2.WithContext(contextWithAPI(req2.Context(), models.APIContext{KeyID: ""}))
	rr2 := httptest.NewRecorder()
	h(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 for json, got %d", rr2.Code)
	}
	var jsonBody map[string]interface{}
	if err := json.Unmarshal(rr2.Body.Bytes(), &jsonBody); err != nil {
		t.Fatalf("invalid json logs: %v", err)
	}
	if cnt, ok := jsonBody["count"].(float64); !ok || int(cnt) != 2 {
		t.Fatalf("expected json count=2, got %#v", jsonBody["count"])
	}
}

// contextWithAPI is a small helper to inject APIContext into request context using
// the package's unexported key. Tests are in the same package so they can use it.
func contextWithAPI(ctx context.Context, a models.APIContext) context.Context {
	return context.WithValue(ctx, apiContextKey, a)
}
