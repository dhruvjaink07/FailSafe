package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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
	http.HandleFunc("/experiment/start", handlers.ExperimentStartHandler(orch))
	http.HandleFunc("/experiment/get", handlers.ExperimentGetHandler(orch))
	http.HandleFunc("/experiment/stop", handlers.ExperimentStopHandler(orch))
	http.HandleFunc("/experiment/metrics", handlers.ExperimentMetricsHandler(orch))
	http.HandleFunc("/experiment/metrics/backend", handlers.ExperimentBackendMetricsHandler(orch))
	http.HandleFunc("/experiment/metrics/android", handlers.ExperimentAndroidMetricsHandler(orch))
	http.HandleFunc("/experiment/android/status", handlers.ExperimentAndroidStatusHandler(orch))
	http.HandleFunc("/upload/apk", handlers.UploadAPKHandler())
	http.HandleFunc("/scenarios/presets", handlers.ScenarioPresetsHandler())
	http.HandleFunc("/frontend/metrics", handlers.FrontendMetricsHandler(orch))

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
