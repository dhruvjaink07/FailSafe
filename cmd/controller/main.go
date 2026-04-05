package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/handlers"
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

	dockerManager := docker.NewManager()

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

	http.HandleFunc("/health", handlers.HealthHandler())
	http.HandleFunc("/upload/apk", handlers.UploadAPKHandler())
	http.HandleFunc("/scenarios/presets", handlers.ScenarioPresetsHandler())
	http.HandleFunc("/frontend/metrics", handlers.FrontendMetricsHandler(orch))

	startRoles := []string{"engineer", "admin"}
	metricsRoles := []string{"viewer", "engineer", "admin"}
	keyCreateRoles := []string{"engineer", "admin"}
	wrap := func(action string, roles []string, next http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireAPIKey(db, roles, action, next)
	}
	_ = keyCreateRoles
	http.HandleFunc("/internal/api-keys/create", handlers.CreateAPIKeyHandler(db, os.Getenv("API_KEY_BOOTSTRAP_TOKEN")))
	http.HandleFunc("/environment/containers", wrap("list_containers", metricsRoles, handlers.DockerContainersListHandler(dockerManager)))
	http.HandleFunc("/environment/containers/start", wrap("start_container", startRoles, handlers.DockerContainerStartHandler(dockerManager)))

	// Platform-scoped routes for lifecycle/status/metrics.
	http.HandleFunc("/experiments/backend/start", wrap("start_experiment", startRoles, handlers.ExperimentBackendStartHandler(orch)))
	http.HandleFunc("/experiments/backend/status", handlers.ExperimentBackendStatusHandler(orch))
	http.HandleFunc("/experiments/backend/stop", wrap("stop_experiment", startRoles, handlers.ExperimentBackendStopHandler(orch)))
	http.HandleFunc("/experiments/backend/metrics", wrap("read_metrics", metricsRoles, handlers.ExperimentBackendMetricsHandler(orch)))

	http.HandleFunc("/experiments/android/start", wrap("start_experiment", startRoles, handlers.ExperimentAndroidStartHandler(orch)))
	http.HandleFunc("/experiments/android/status", handlers.ExperimentAndroidStatusHandler(orch))
	http.HandleFunc("/experiments/android/stop", wrap("stop_experiment", startRoles, handlers.ExperimentAndroidStopHandler(orch)))
	http.HandleFunc("/experiments/android/metrics", wrap("read_metrics", metricsRoles, handlers.ExperimentAndroidMetricsHandler(orch)))

	http.HandleFunc("/experiments/frontend/start", wrap("start_experiment", startRoles, handlers.ExperimentFrontendStartHandler(orch)))
	http.HandleFunc("/experiments/frontend/status", handlers.ExperimentFrontendStatusHandler(orch))
	http.HandleFunc("/experiments/frontend/stop", wrap("stop_experiment", startRoles, handlers.ExperimentFrontendStopHandler(orch)))
	http.HandleFunc("/experiments/frontend/metrics", wrap("read_metrics", metricsRoles, handlers.ExperimentFrontendMetricsHandler(orch)))
	http.HandleFunc("/experiments/frontend/fault-command", handlers.ExperimentFrontendFaultCommandHandler(orch))

	log.Println("Server running on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
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
