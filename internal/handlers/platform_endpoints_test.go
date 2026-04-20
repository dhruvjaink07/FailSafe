package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
)

type fakeExperimentService struct {
	mu sync.Mutex

	nextID int

	experiments    map[string]*models.Experiment
	backendMetrics map[string]interface{}
	androidMetrics map[string]interface{}
	androidStatus  map[string]interface{}
	stopped        map[string]bool
}

func newFakeExperimentService() *fakeExperimentService {
	return &fakeExperimentService{
		experiments: make(map[string]*models.Experiment),
		backendMetrics: map[string]interface{}{
			"endpoints": map[string]interface{}{
				"svc-a": map[string]interface{}{"requests_total": 5, "degraded": false},
			},
			"blast_radius_percent": 0,
			"cascade_depth":        0,
			"system_severity":      "isolated",
			"resilience_threshold": map[string]interface{}{},
			"timeline":             []interface{}{},
		},
		androidMetrics: map[string]interface{}{
			"health":    map[string]interface{}{"score": 98},
			"recovery":  map[string]interface{}{"recovered": true},
			"stability": map[string]interface{}{"status": "stable"},
			"timeline":  []interface{}{},
		},
		androidStatus: map[string]interface{}{
			"state": "running",
			"phase": "injecting",
		},
		stopped: make(map[string]bool),
	}
}

func (f *fakeExperimentService) StartExperiment(
	faultType string,
	targets []string,
	targetType string,
	observedEndpoints []string,
	observationType string,
	duration int,
	adaptive bool,
	stepIntensity int,
	maxIntensity int,
	deps models.DependencyGraph,
	targetMap map[string][]string,
	scheduledFaults []models.ScheduledFault,
	expected models.ExpectedState,
	androidOptions *orchestrator.AndroidRunOptions,
	androidApp *orchestrator.AndroidAppConfig,
	frontendRun *models.FrontendRunConfig,
	apiCtx *models.APIContext,
) (*models.Experiment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	id := fmt.Sprintf("fake-%d", f.nextID)
	exp := &models.Experiment{
		ID:                id,
		FaultType:         faultType,
		Targets:           append([]string(nil), targets...),
		TargetType:        targetType,
		ObservedEndpoints: append([]string(nil), observedEndpoints...),
		ObservationType:   observationType,
		Duration:          duration,
		Adaptive:          adaptive,
		StepIntensity:     stepIntensity,
		MaxIntensity:      maxIntensity,
		DependencyGraph:   deps,
		TargetEndpointMap: targetMap,
		Scenario:          scheduledFaults,
		Expected:          expected,
		State:             models.StateRunning,
		Phase:             models.PhaseBaseline,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		TimelineHistory:   make(map[int]models.IntensityTimeline),
	}
	if frontendRun != nil {
		exp.FrontendRun = frontendRun
	}
	f.experiments[id] = exp
	return exp, nil
}

func (f *fakeExperimentService) GetExperiment(id string) (map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	exp, ok := f.experiments[id]
	if !ok {
		return nil, fmt.Errorf("experiment not found")
	}
	return map[string]interface{}{"experiment": exp}, nil
}

func (f *fakeExperimentService) StopExperiment(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.experiments[id]; !ok {
		return fmt.Errorf("experiment not found")
	}
	f.stopped[id] = true
	return nil
}

func (f *fakeExperimentService) GetBackendMetrics(id string) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.experiments[id]; !ok {
		return nil, fmt.Errorf("experiment not found")
	}
	return f.backendMetrics, nil
}

func (f *fakeExperimentService) GetAndroidMetrics(id string) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.experiments[id]; !ok {
		return nil, fmt.Errorf("experiment not found")
	}
	return f.androidMetrics, nil
}

func (f *fakeExperimentService) GetAndroidStatus(id string) (map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.experiments[id]; !ok {
		return nil, fmt.Errorf("experiment not found")
	}
	return f.androidStatus, nil
}

func (f *fakeExperimentService) GetExperimentTargetType(id string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	exp, ok := f.experiments[id]
	if !ok {
		return "", fmt.Errorf("experiment not found")
	}
	return exp.TargetType, nil
}

func (f *fakeExperimentService) GetFrontendMetrics(id string) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.experiments[id]; !ok {
		return nil, fmt.Errorf("experiment not found")
	}
	return map[string]interface{}{
		"frontend":       []map[string]interface{}{{"experiment_id": id}},
		"frontend_score": map[string]interface{}{"score": 99.0, "status": "stable"},
		"failsafe_index": map[string]interface{}{"score": 99.0},
	}, nil
}

func (f *fakeExperimentService) GetFrontendFaultCommand(id string) (map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.experiments[id]; !ok {
		return nil, fmt.Errorf("experiment not found")
	}
	return map[string]interface{}{"active": false}, nil
}

func (f *fakeExperimentService) GetExperimentHistory(apiKeyID string, limit int, offset int) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (f *fakeExperimentService) GetExperimentHistoryCount(apiKeyID string) (int, error) {
	return 0, nil
}

func (f *fakeExperimentService) GetExperimentHistoryDetail(apiKeyID string, experimentID string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (f *fakeExperimentService) GetBackendLogs(apiKeyID string, experimentID string, tail int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.experiments[experimentID]; !ok {
		return "", fmt.Errorf("experiment not found")
	}
	return "[fake-log] container started\n[fake-log] injected fault\n[fake-log] recovered\n", nil
}

func (f *fakeExperimentService) StreamBackendLogs(apiKeyID string, experimentID string, tail int, w http.ResponseWriter, format string, follow bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.experiments[experimentID]; !ok {
		return fmt.Errorf("experiment not found")
	}
	logs := "[fake-log] container started\n[fake-log] injected fault\n[fake-log] recovered\n"
	if format == "json" {
		lines := strings.Split(logs, "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		return json.NewEncoder(w).Encode(map[string]interface{}{"lines": lines, "count": len(lines)})
	}
	_, err := w.Write([]byte(logs))
	return err
}

func (f *fakeExperimentService) AddFrontendMetrics(data []models.FrontendMetrics) {}

func (f *fakeExperimentService) GetLatestSystemMetrics() (map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return map[string]interface{}{
		"backend":  f.backendMetrics,
		"android":  f.androidMetrics,
		"frontend": map[string]interface{}{"failsafe_index": map[string]interface{}{"score": 99.0}},
	}, nil
}

func (f *fakeExperimentService) ListExperiments() ([]*models.Experiment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*models.Experiment, 0, len(f.experiments))
	for _, e := range f.experiments {
		out = append(out, e)
	}
	return out, nil
}

func newTestMux(orch ExperimentService) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/upload/apk", UploadAPKHandler())
	mux.HandleFunc("/scenarios/presets", ScenarioPresetsHandler())
	mux.HandleFunc("/frontend/metrics", FrontendMetricsHandler(orch))

	mux.HandleFunc("/experiments/backend/start", ExperimentBackendStartHandler(orch))
	mux.HandleFunc("/experiments/backend/status", ExperimentBackendStatusHandler(orch))
	mux.HandleFunc("/experiments/backend/stop", ExperimentBackendStopHandler(orch))
	mux.HandleFunc("/experiments/backend/metrics", ExperimentBackendMetricsHandler(orch))
	mux.HandleFunc("/experiments/backend/logs", ExperimentBackendLogsHandler(orch))

	mux.HandleFunc("/experiments/android/start", ExperimentAndroidStartHandler(orch))
	mux.HandleFunc("/experiments/android/status", ExperimentAndroidStatusHandler(orch))
	mux.HandleFunc("/experiments/android/stop", ExperimentAndroidStopHandler(orch))
	mux.HandleFunc("/experiments/android/metrics", ExperimentAndroidMetricsHandler(orch))

	mux.HandleFunc("/experiments/frontend/start", ExperimentFrontendStartHandler(orch))
	mux.HandleFunc("/experiments/frontend/status", ExperimentFrontendStatusHandler(orch))
	mux.HandleFunc("/experiments/frontend/stop", ExperimentFrontendStopHandler(orch))
	mux.HandleFunc("/experiments/frontend/metrics", ExperimentFrontendMetricsHandler(orch))
	mux.HandleFunc("/experiments/frontend/fault-command", ExperimentFrontendFaultCommandHandler(orch))

	return mux
}

func TestLegacyExperimentRoutesRemoved(t *testing.T) {
	mux := newTestMux(nil)

	for _, path := range []string{
		"/experiment/start",
		"/experiment/get?id=x",
		"/experiment/stop?id=x",
		"/experiment/metrics?id=x",
		"/experiment/metrics/backend?id=x",
		"/experiment/metrics/android?id=x",
		"/experiment/android/status?id=x",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for legacy route %s, got %d", path, res.Code)
		}
	}
}

func TestBackendLifecycleWithFakeService(t *testing.T) {
	fake := newFakeExperimentService()
	mux := newTestMux(fake)

	startReq := mustJSONRequest(t, http.MethodPost, "/experiments/backend/start", map[string]interface{}{
		"fault_type":  "network_delay",
		"targets":     []string{"svc-a"},
		"target_type": "docker",
		"duration":    20,
	})
	startRes := httptest.NewRecorder()
	mux.ServeHTTP(startRes, startReq)
	if startRes.Code != http.StatusOK {
		t.Fatalf("expected backend start 200, got %d: %s", startRes.Code, startRes.Body.String())
	}

	var started models.Experiment
	if err := json.Unmarshal(startRes.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode backend start: %v", err)
	}

	statusRes := httptest.NewRecorder()
	statusReq := httptest.NewRequest(http.MethodGet, "/experiments/backend/status?id="+started.ID, nil)
	mux.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("expected backend status 200, got %d: %s", statusRes.Code, statusRes.Body.String())
	}

	metricsRes := httptest.NewRecorder()
	metricsReq := httptest.NewRequest(http.MethodGet, "/experiments/backend/metrics?id="+started.ID, nil)
	mux.ServeHTTP(metricsRes, metricsReq)
	if metricsRes.Code != http.StatusOK {
		t.Fatalf("expected backend metrics 200, got %d: %s", metricsRes.Code, metricsRes.Body.String())
	}

	stopRes := httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/experiments/backend/stop?id="+started.ID, nil)
	mux.ServeHTTP(stopRes, stopReq)
	if stopRes.Code != http.StatusOK {
		t.Fatalf("expected backend stop 200, got %d: %s", stopRes.Code, stopRes.Body.String())
	}

	// Smoke-test logs endpoint for started experiment
	logsRes := httptest.NewRecorder()
	logsReq := httptest.NewRequest(http.MethodGet, "/experiments/backend/logs?id="+started.ID, nil)
	// Attach a fake API context so handler authorizes the request in tests
	logsReq = logsReq.WithContext(context.WithValue(logsReq.Context(), apiContextKey, models.APIContext{KeyID: "public"}))
	mux.ServeHTTP(logsRes, logsReq)
	if logsRes.Code != http.StatusOK {
		t.Fatalf("expected backend logs 200, got %d: %s", logsRes.Code, logsRes.Body.String())
	}
	if !strings.Contains(logsRes.Body.String(), "[fake-log]") {
		t.Fatalf("expected fake log content, got: %s", logsRes.Body.String())
	}
}

func TestAndroidLifecycleWithFakeService(t *testing.T) {
	fake := newFakeExperimentService()
	mux := newTestMux(fake)

	startReq := mustJSONRequest(t, http.MethodPost, "/experiments/android/start", map[string]interface{}{
		"fault_type":  "kill_app",
		"targets":     []string{"com.example.code"},
		"target_type": "android",
		"duration":    20,
	})
	startRes := httptest.NewRecorder()
	mux.ServeHTTP(startRes, startReq)
	if startRes.Code != http.StatusOK {
		t.Fatalf("expected android start 200, got %d: %s", startRes.Code, startRes.Body.String())
	}

	var started models.Experiment
	if err := json.Unmarshal(startRes.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode android start: %v", err)
	}

	statusRes := httptest.NewRecorder()
	statusReq := httptest.NewRequest(http.MethodGet, "/experiments/android/status?id="+started.ID, nil)
	mux.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("expected android status 200, got %d: %s", statusRes.Code, statusRes.Body.String())
	}

	metricsRes := httptest.NewRecorder()
	metricsReq := httptest.NewRequest(http.MethodGet, "/experiments/android/metrics?id="+started.ID, nil)
	mux.ServeHTTP(metricsRes, metricsReq)
	if metricsRes.Code != http.StatusOK {
		t.Fatalf("expected android metrics 200, got %d: %s", metricsRes.Code, metricsRes.Body.String())
	}

	stopRes := httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/experiments/android/stop?id="+started.ID, nil)
	mux.ServeHTTP(stopRes, stopReq)
	if stopRes.Code != http.StatusOK {
		t.Fatalf("expected android stop 200, got %d: %s", stopRes.Code, stopRes.Body.String())
	}
}

func TestPlatformEndpointsValidation(t *testing.T) {
	mux := newTestMux(nil)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		code   int
	}{
		{name: "backend start invalid json", method: http.MethodPost, path: "/experiments/backend/start", body: "{", code: http.StatusBadRequest},
		{name: "backend status missing id", method: http.MethodGet, path: "/experiments/backend/status", code: http.StatusBadRequest},
		{name: "backend stop missing id", method: http.MethodPost, path: "/experiments/backend/stop", code: http.StatusBadRequest},
		{name: "backend metrics missing id", method: http.MethodGet, path: "/experiments/backend/metrics", code: http.StatusBadRequest},
		{name: "backend logs missing id", method: http.MethodGet, path: "/experiments/backend/logs", code: http.StatusBadRequest},
		{name: "android start invalid json", method: http.MethodPost, path: "/experiments/android/start", body: "{", code: http.StatusBadRequest},
		{name: "android status missing id", method: http.MethodGet, path: "/experiments/android/status", code: http.StatusBadRequest},
		{name: "android stop missing id", method: http.MethodPost, path: "/experiments/android/stop", code: http.StatusBadRequest},
		{name: "android metrics missing id", method: http.MethodGet, path: "/experiments/android/metrics", code: http.StatusBadRequest},
		{name: "frontend start invalid json", method: http.MethodPost, path: "/experiments/frontend/start", body: "{", code: http.StatusBadRequest},
		{name: "frontend status missing id", method: http.MethodGet, path: "/experiments/frontend/status", code: http.StatusBadRequest},
		{name: "frontend stop missing id", method: http.MethodPost, path: "/experiments/frontend/stop", code: http.StatusBadRequest},
		{name: "frontend metrics missing id", method: http.MethodGet, path: "/experiments/frontend/metrics", code: http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body *bytes.Reader
			if tc.body != "" {
				body = bytes.NewReader([]byte(tc.body))
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != tc.code {
				t.Fatalf("expected status %d, got %d", tc.code, res.Code)
			}
		})
	}
}

func TestFrontendLifecycleConnectivity(t *testing.T) {
	orch := orchestrator.NewOrchestrator(nil, "", "", "", "", "")
	mux := newTestMux(orch)

	startBody := map[string]interface{}{
		"fault_type":   "latency",
		"target_type":  "frontend",
		"duration":     20,
		"frontend_run": map[string]interface{}{"base_url": "https://example.com"},
	}
	startReq := mustJSONRequest(t, http.MethodPost, "/experiments/frontend/start", startBody)
	startRes := httptest.NewRecorder()
	mux.ServeHTTP(startRes, startReq)
	if startRes.Code != http.StatusOK {
		t.Fatalf("expected start 200, got %d: %s", startRes.Code, startRes.Body.String())
	}

	var started models.Experiment
	if err := json.Unmarshal(startRes.Body.Bytes(), &started); err != nil {
		t.Fatalf("failed to decode start response: %v", err)
	}
	if started.ID == "" {
		t.Fatal("missing experiment id in start response")
	}

	statusRes := httptest.NewRecorder()
	statusReq := httptest.NewRequest(http.MethodGet, "/experiments/frontend/status?id="+started.ID, nil)
	mux.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", statusRes.Code, statusRes.Body.String())
	}

	batchBody := map[string]interface{}{
		"metrics": []map[string]interface{}{
			{
				"experiment_id": started.ID,
				"phase":         "baseline",
				"page":          "/",
				"metrics": map[string]interface{}{
					"lcp":                  1200,
					"cls":                  0.04,
					"inp":                  90,
					"long_tasks":           1,
					"errors":               0,
					"unhandled_rejections": 0,
				},
				"api_calls": []interface{}{},
				"timestamp": time.Now().UnixMilli(),
			},
		},
	}
	batchReq := mustJSONRequest(t, http.MethodPost, "/frontend/metrics", batchBody)
	batchRes := httptest.NewRecorder()
	mux.ServeHTTP(batchRes, batchReq)
	if batchRes.Code != http.StatusOK {
		t.Fatalf("expected collector ingest 200, got %d: %s", batchRes.Code, batchRes.Body.String())
	}

	metricsRes := httptest.NewRecorder()
	metricsReq := httptest.NewRequest(http.MethodGet, "/experiments/frontend/metrics?id="+started.ID, nil)
	mux.ServeHTTP(metricsRes, metricsReq)
	if metricsRes.Code != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d: %s", metricsRes.Code, metricsRes.Body.String())
	}

	commandRes := httptest.NewRecorder()
	commandReq := httptest.NewRequest(http.MethodGet, "/experiments/frontend/fault-command?id="+started.ID, nil)
	mux.ServeHTTP(commandRes, commandReq)
	if commandRes.Code != http.StatusOK {
		t.Fatalf("expected frontend fault-command 200, got %d: %s", commandRes.Code, commandRes.Body.String())
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(metricsRes.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("failed to decode metrics response: %v", err)
	}
	frontendRaw, ok := metrics["frontend"].([]interface{})
	if !ok {
		t.Fatalf("frontend metrics payload missing frontend array: %s", metricsRes.Body.String())
	}
	if len(frontendRaw) == 0 {
		t.Fatal("expected at least one frontend metric sample")
	}

	stopRes := httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/experiments/frontend/stop?id="+started.ID, nil)
	mux.ServeHTTP(stopRes, stopReq)
	if stopRes.Code != http.StatusOK {
		t.Fatalf("expected stop 200, got %d: %s", stopRes.Code, stopRes.Body.String())
	}
}

func TestFrontendFaultCommandActiveDuringInjecting(t *testing.T) {
	orch := orchestrator.NewOrchestrator(nil, "", "", "", "", "")
	mux := newTestMux(orch)

	startBody := map[string]interface{}{
		"fault_type":   "network_delay",
		"target_type":  "frontend",
		"duration":     20,
		"frontend_run": map[string]interface{}{"base_url": "https://example.com"},
	}
	startReq := mustJSONRequest(t, http.MethodPost, "/experiments/frontend/start", startBody)
	startRes := httptest.NewRecorder()
	mux.ServeHTTP(startRes, startReq)
	if startRes.Code != http.StatusOK {
		t.Fatalf("expected start 200, got %d: %s", startRes.Code, startRes.Body.String())
	}

	var started models.Experiment
	if err := json.Unmarshal(startRes.Body.Bytes(), &started); err != nil {
		t.Fatalf("failed to decode start response: %v", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	activeSeen := false

	for time.Now().Before(deadline) {
		statusRes := httptest.NewRecorder()
		statusReq := httptest.NewRequest(http.MethodGet, "/experiments/frontend/status?id="+started.ID, nil)
		mux.ServeHTTP(statusRes, statusReq)
		if statusRes.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", statusRes.Code, statusRes.Body.String())
		}

		commandRes := httptest.NewRecorder()
		commandReq := httptest.NewRequest(http.MethodGet, "/experiments/frontend/fault-command?id="+started.ID, nil)
		mux.ServeHTTP(commandRes, commandReq)
		if commandRes.Code != http.StatusOK {
			t.Fatalf("expected frontend fault-command 200, got %d: %s", commandRes.Code, commandRes.Body.String())
		}

		var commandPayload map[string]interface{}
		if err := json.Unmarshal(commandRes.Body.Bytes(), &commandPayload); err != nil {
			t.Fatalf("failed to decode command response: %v", err)
		}

		if active, ok := commandPayload["active"].(bool); ok && active {
			activeSeen = true
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	if !activeSeen {
		t.Fatalf("expected active frontend fault command during injecting window for experiment %s", started.ID)
	}

	stopRes := httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/experiments/frontend/stop?id="+started.ID, nil)
	mux.ServeHTTP(stopRes, stopReq)
	if stopRes.Code != http.StatusOK {
		t.Fatalf("expected stop 200, got %d: %s", stopRes.Code, stopRes.Body.String())
	}
}

func mustJSONRequest(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	return req
}
