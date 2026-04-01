package handlers

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
)

func ExperimentStartHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		var appCfg *orchestrator.AndroidAppConfig
		if strings.EqualFold(req.TargetType, "android") && strings.TrimSpace(req.APK) != "" {
			apkMeta, ok := getUploadedAPK(req.APK)
			if !ok {
				http.Error(w, "invalid apk reference: upload id not found", http.StatusBadRequest)
				return
			}
			appCfg = &orchestrator.AndroidAppConfig{
				APKPath:  apkMeta.Path,
				Package:  apkMeta.Package,
				Activity: apkMeta.Activity,
			}
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
			appCfg,
		)

		log.Printf("REQ: %+v\n", req)
		if err != nil {
			log.Println("StartExperiment error:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(exp)
	}
}

func ExperimentGetHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		_ = json.NewEncoder(w).Encode(exp)
	}
}

func ExperimentStopHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		_, _ = w.Write([]byte("experiment stopped"))
	}
}

func ExperimentMetricsHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		_ = json.NewEncoder(w).Encode(data)
	}
}

func ExperimentBackendMetricsHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		data, err := orch.GetBackendMetrics(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(data)
	}
}

func ExperimentAndroidMetricsHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		data, err := orch.GetAndroidMetrics(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(data)
	}
}

func ExperimentAndroidStatusHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(status)
	}
}

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
	APK               string
}

type AndroidRunRequest struct {
	AVDName       string
	Headless      *bool
	ResetAppState *bool
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
		AVDName:       strings.TrimSpace(a.AVDName),
		Headless:      headless,
		ResetAppState: a.ResetAppState != nil && *a.ResetAppState,
	}
}

func (s *StartRequest) UnmarshalJSON(data []byte) error {
	type androidRunAlias struct {
		AVDName      string `json:"avdName"`
		AVDNameSnake string `json:"avd_name"`
		Headless     *bool  `json:"headless"`
		Background   *bool  `json:"background"`
		ResetApp     *bool  `json:"resetAppState"`
		ResetAppSnk  *bool  `json:"reset_app_state"`
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
		APK               string                  `json:"apk"`
		APKSnake          string                  `json:"apk_id"`
		UploadedAPKID     string                  `json:"uploadedApkId"`
		UploadedAPKSnake  string                  `json:"uploaded_apk_id"`
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
	s.APK = firstNonEmpty(a.APK, a.APKSnake, a.UploadedAPKID, a.UploadedAPKSnake)

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
			AVDName:       firstNonEmpty(runCfg.AVDName, runCfg.AVDNameSnake),
			Headless:      headless,
			ResetAppState: firstNonNilBool(runCfg.ResetApp, runCfg.ResetAppSnk),
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

func firstNonNilBool(values ...*bool) *bool {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
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
