package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
	"github.com/dhruvjaink07/failsafe/internal/storage"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("warning: failed to load .env: %v", err)
	}

	if err := validateEnv(); err != nil {
		log.Fatal(err)
	}

	connStr := os.Getenv("DB_URL")

	db, err := storage.NewPostgres(connStr)
	if err != nil {
		panic(err)
	}
	// dm := docker.NewManager()
	// injector := fault.NewDockerInjector(dm)
	orch := orchestrator.NewOrchestrator(
		db,
		os.Getenv("CONFIG_PARAM_1"),
		os.Getenv("CONFIG_PARAM_2"),
		os.Getenv("CONFIG_PARAM_3"),
		os.Getenv("CONFIG_PARAM_4"),
		os.Getenv("CONFIG_PARAM_5"),
	)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/experiment/start", func(w http.ResponseWriter, r *http.Request) {
		startHandler(w, r, orch)
	})
	http.HandleFunc("/experiment/get", func(w http.ResponseWriter, r *http.Request) {
		getHandler(w, r, orch)
	})
	http.HandleFunc("/experiment/stop", func(w http.ResponseWriter, r *http.Request) {
		stopHandler(w, r, orch)
	})
	http.HandleFunc("/experiment/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricsHandler(w, r, orch)
	})
	http.HandleFunc("/experiment/android/status", func(w http.ResponseWriter, r *http.Request) {
		androidStatusHandler(w, r, orch)
	})
	http.HandleFunc("/scenarios/presets", func(w http.ResponseWriter, r *http.Request) {
		presetsHandler(w, r)
	})

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadDotEnv(path string) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(buf), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
			value = value[1 : len(value)-1]
		}

		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}

	return nil
}

func validateEnv() error {
	required := []string{
		"DB_URL",
		"CONFIG_PARAM_1",
		"CONFIG_PARAM_2",
		"CONFIG_PARAM_3",
		"CONFIG_PARAM_4",
		"CONFIG_PARAM_5",
	}

	missing := make([]string, 0)
	for _, k := range required {
		if strings.TrimSpace(os.Getenv(k)) == "" {
			missing = append(missing, k)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

/*
---------------------------------------
Request Models
---------------------------------------
*/

type StartRequest struct {
	FaultType         string
	Targets           []string
	TargetType        string
	ObservedEndpoints []string
	ObservationType   string
	Duration          int

	Adaptive        bool
	StepIntensity   int
	MaxIntensity    int
	DependencyGraph map[string][]string

	TargetEndpointMap map[string][]string
	Scenario          []models.ScheduledFault
	ScenarioPreset    string
	Expected          models.ExpectedState
	AndroidRun        *AndroidRunRequest
}

type AndroidRunRequest struct {
	AVDName  string
	Headless *bool
}

func (a *AndroidRunRequest) toOptions() *orchestrator.AndroidRunOptions {
	if a == nil {
		return nil
	}
	headless := true
	if a.Headless != nil {
		headless = *a.Headless
	}
	return &orchestrator.AndroidRunOptions{
		AVDName:  strings.TrimSpace(a.AVDName),
		Headless: headless,
	}
}

func (s *StartRequest) UnmarshalJSON(data []byte) error {
	type androidRunAlias struct {
		AVDName      string `json:"avdName"`
		AVDNameSnake string `json:"avd_name"`
		Headless     *bool  `json:"headless"`
		Background   *bool  `json:"background"`
	}

	type Alias struct {
		FaultType         string                  `json:"faultType"`
		FaultTypeSnake    string                  `json:"fault_type"`
		Targets           []string                `json:"targets"`
		TargetType        string                  `json:"targetType"`
		TargetTypeSnake   string                  `json:"target_type"`
		ObservedEndpoints []string                `json:"observedEndpoints"`
		ObservedSnake     []string                `json:"observed_endpoints"`
		ObservationType   string                  `json:"observationType"`
		ObservationSnake  string                  `json:"observation_type"`
		Duration          int                     `json:"duration"`
		DurationSeconds   int                     `json:"duration_seconds"`
		Adaptive          *bool                   `json:"adaptive"`
		StepIntensity     int                     `json:"stepIntensity"`
		StepIntensitySnk  int                     `json:"step_intensity"`
		MaxIntensity      int                     `json:"maxIntensity"`
		MaxIntensitySnk   int                     `json:"max_intensity"`
		DependencyGraph   map[string][]string     `json:"dependencyGraph"`
		DependencySnk     map[string][]string     `json:"dependency_graph"`
		TargetEndpointMap map[string][]string     `json:"targetEndpointMap"`
		TargetEndpointSnk map[string][]string     `json:"target_endpoint_map"`
		Scenario          []models.ScheduledFault `json:"scenario"`
		Scenarios         []models.ScheduledFault `json:"scenarios"`
		ScenarioPreset    string                  `json:"scenarioPreset"`
		ScenarioPresetSnk string                  `json:"scenario_preset"`
		Expected          models.ExpectedState    `json:"expected"`
		AndroidRun        *androidRunAlias        `json:"androidRun"`
		AndroidRunSnake   *androidRunAlias        `json:"android_run"`
	}

	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	s.FaultType = firstNonEmpty(a.FaultType, a.FaultTypeSnake)
	s.Targets = a.Targets
	s.TargetType = firstNonEmpty(a.TargetType, a.TargetTypeSnake)
	if len(a.ObservedEndpoints) > 0 {
		s.ObservedEndpoints = a.ObservedEndpoints
	} else {
		s.ObservedEndpoints = a.ObservedSnake
	}
	s.ObservationType = firstNonEmpty(a.ObservationType, a.ObservationSnake)
	if a.Duration != 0 {
		s.Duration = a.Duration
	} else {
		s.Duration = a.DurationSeconds
	}
	if a.Adaptive != nil {
		s.Adaptive = *a.Adaptive
	}
	if a.StepIntensity != 0 {
		s.StepIntensity = a.StepIntensity
	} else {
		s.StepIntensity = a.StepIntensitySnk
	}
	if a.MaxIntensity != 0 {
		s.MaxIntensity = a.MaxIntensity
	} else {
		s.MaxIntensity = a.MaxIntensitySnk
	}
	if len(a.DependencyGraph) > 0 {
		s.DependencyGraph = a.DependencyGraph
	} else {
		s.DependencyGraph = a.DependencySnk
	}
	if len(a.TargetEndpointMap) > 0 {
		s.TargetEndpointMap = a.TargetEndpointMap
	} else {
		s.TargetEndpointMap = a.TargetEndpointSnk
	}
	if len(a.Scenario) > 0 {
		s.Scenario = a.Scenario
	} else {
		s.Scenario = a.Scenarios
	}
	s.ScenarioPreset = firstNonEmpty(a.ScenarioPreset, a.ScenarioPresetSnk)
	s.Expected = a.Expected

	runCfg := a.AndroidRun
	if runCfg == nil {
		runCfg = a.AndroidRunSnake
	}
	if runCfg != nil {
		headless := runCfg.Headless
		if headless == nil && runCfg.Background != nil {
			headless = runCfg.Background
		}
		s.AndroidRun = &AndroidRunRequest{
			AVDName:  firstNonEmpty(runCfg.AVDName, runCfg.AVDNameSnake),
			Headless: headless,
		}
	}

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func applyScenarioPreset(req *StartRequest) error {
	if req.ScenarioPreset == "" || len(req.Scenario) > 0 {
		return nil
	}

	preset := strings.TrimSpace(req.ScenarioPreset)
	if preset == "" {
		return nil
	}

	path := filepath.Join("configs", "scenarios", "android", preset+".json")
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to load scenario preset %q: %w", preset, err)
	}

	var scenario []models.ScheduledFault
	if err := json.Unmarshal(buf, &scenario); err != nil {
		return fmt.Errorf("invalid scenario preset %q: %w", preset, err)
	}

	req.Scenario = scenario
	return nil
}

/*
---------------------------------------
Handlers
---------------------------------------
*/

func startHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

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
		log.Println("preset load error:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.FaultType == "" || len(req.Targets) == 0 {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
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
	)

	log.Printf("REQ: %+v\n", req)
	if err != nil {
		log.Println("StartExperiment error:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(exp)
}

func getHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	exp, err := orch.GetExperiment(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(exp)
}

func stopHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	if err := orch.StopExperiment(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write([]byte("experiment stopped"))
}

func metricsHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	data, err := orch.GetMetrics(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(data)
}

func androidStatusHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	status, err := orch.GetAndroidStatus(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func presetsHandler(w http.ResponseWriter, r *http.Request) {
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
	json.NewEncoder(w).Encode(response)
}
