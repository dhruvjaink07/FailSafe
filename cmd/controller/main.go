package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/handlers"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
	"github.com/dhruvjaink07/failsafe/internal/preview"
	"github.com/dhruvjaink07/failsafe/internal/storage"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("warning: failed to load .env: %v", err)
	}

	ensureDefaultConfigParams()

	connStr, err := resolveDBConnString()
	if err != nil {
		log.Fatal(err)
	}

	if err := validateEnv(); err != nil {
		log.Fatal(err)
	}

	dockerManager := docker.NewManager()

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
	// API key creation supports optional JWT auth for user-owned keys
	http.HandleFunc("/internal/api-keys/create", handlers.JWTMiddleware(handlers.CreateAPIKeyHandler(db, os.Getenv("API_KEY_BOOTSTRAP_TOKEN"))))
	http.HandleFunc("/internal/auth/signup", handlers.SignupHandler(db))
	http.HandleFunc("/internal/auth/signin", handlers.SigninHandler(db))
	http.HandleFunc("/internal/api-keys", wrap("list_api_keys", metricsRoles, handlers.ListAPIKeysHandler(db)))
	http.HandleFunc("/internal/api-keys/revoke", wrap("revoke_api_key", []string{"admin"}, handlers.RevokeAPIKeyHandler(db)))
	http.HandleFunc("/internal/api-keys/rotate", wrap("rotate_api_key", []string{"admin"}, handlers.RotateAPIKeyHandler(db)))
	http.HandleFunc("/environment/containers", wrap("list_containers", metricsRoles, handlers.DockerContainersListHandler(dockerManager)))
	http.HandleFunc("/environment/containers/start", wrap("start_container", startRoles, handlers.DockerContainerStartHandler(dockerManager)))
	http.HandleFunc("/environment/docker/engine/start", wrap("start_docker_engine", startRoles, handlers.DockerEngineStartHandler(dockerManager)))

	// Platform-scoped routes for lifecycle/status/metrics.
	http.HandleFunc("/experiments/backend/start", wrap("start_experiment", startRoles, handlers.ExperimentBackendStartHandler(orch)))
	http.HandleFunc("/experiments/backend/status", handlers.ExperimentBackendStatusHandler(orch))
	http.HandleFunc("/experiments/backend/stop", wrap("stop_experiment", startRoles, handlers.ExperimentBackendStopHandler(orch)))
	http.HandleFunc("/experiments/backend/metrics", wrap("read_metrics", metricsRoles, handlers.ExperimentBackendMetricsHandler(orch)))
	http.HandleFunc("/experiments/backend/logs", wrap("read_logs", metricsRoles, handlers.ExperimentBackendLogsHandler(orch)))

	http.HandleFunc("/experiments/android/start", wrap("start_experiment", startRoles, handlers.ExperimentAndroidStartHandler(orch)))
	http.HandleFunc("/experiments/android/status", handlers.ExperimentAndroidStatusHandler(orch))
	http.HandleFunc("/experiments/android/stop", wrap("stop_experiment", startRoles, handlers.ExperimentAndroidStopHandler(orch)))
	http.HandleFunc("/experiments/android/metrics", wrap("read_metrics", metricsRoles, handlers.ExperimentAndroidMetricsHandler(orch)))
	http.HandleFunc("/experiments/android/preview/mjpeg", wrap("preview_mjpeg", metricsRoles, preview.MJPEGPreviewHandler()))
	http.HandleFunc("/experiments/android/preview/start", wrap("preview_start", metricsRoles, func(w http.ResponseWriter, r *http.Request) {
		preview.StartPreviewHandler()(w, r)
	}))
	http.HandleFunc("/experiments/android/preview/stop", wrap("preview_stop", metricsRoles, func(w http.ResponseWriter, r *http.Request) {
		preview.StopPreviewHandler()(w, r)
	}))

	http.HandleFunc("/experiments/android/preview/metrics", wrap("preview_metrics", metricsRoles, preview.SessionMetricsHandler()))
	http.HandleFunc("/experiments/android/preview/sessions", wrap("preview_sessions", metricsRoles, preview.ListSessionsHandler()))

	// Serve the preview HTML client from docs/preview at /preview-client/
	fs := http.FileServer(http.Dir("docs/preview"))
	http.Handle("/preview-client/", http.StripPrefix("/preview-client/", fs))
	http.HandleFunc("/preview-client", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/preview-client/", http.StatusFound)
	})

	// Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/experiments/frontend/start", wrap("start_experiment", startRoles, handlers.ExperimentFrontendStartHandler(orch)))
	http.HandleFunc("/experiments/frontend/status", handlers.ExperimentFrontendStatusHandler(orch))
	http.HandleFunc("/experiments/frontend/stop", wrap("stop_experiment", startRoles, handlers.ExperimentFrontendStopHandler(orch)))
	http.HandleFunc("/experiments/frontend/metrics", wrap("read_metrics", metricsRoles, handlers.ExperimentFrontendMetricsHandler(orch)))
	http.HandleFunc("/experiments/frontend/fault-command", handlers.ExperimentFrontendFaultCommandHandler(orch))
	http.HandleFunc("/metrics/system", wrap("read_metrics", metricsRoles, handlers.SystemMetricsHandler(orch)))
	http.HandleFunc("/experiments", wrap("read_metrics", metricsRoles, handlers.ExperimentsListHandler(orch)))
	http.HandleFunc("/experiments/history", wrap("read_metrics_history", metricsRoles, handlers.ExperimentHistoryHandler(orch)))
	http.HandleFunc("/experiments/history/detail", wrap("read_metrics_history", metricsRoles, handlers.ExperimentHistoryDetailHandler(orch)))

	log.Println("Server running on 127.0.0.1:8000")
	log.Fatal(http.ListenAndServe("127.0.0.1:8000", nil))
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

func ensureDefaultConfigParams() {
	keys := []string{
		"CONFIG_PARAM_1",
		"CONFIG_PARAM_2",
		"CONFIG_PARAM_3",
		"CONFIG_PARAM_4",
		"CONFIG_PARAM_5",
	}

	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			_ = os.Setenv(key, "local")
		}
	}
}

func resolveDBConnString() (string, error) {
	if direct := strings.TrimSpace(os.Getenv("DB_URL")); direct != "" {
		return direct, nil
	}

	host := strings.TrimSpace(os.Getenv("DB_HOST"))
	port := strings.TrimSpace(os.Getenv("DB_PORT"))
	user := strings.TrimSpace(os.Getenv("DB_USER"))
	password := strings.TrimSpace(os.Getenv("DB_PASSWORD"))
	dbName := strings.TrimSpace(os.Getenv("DB_NAME"))

	missing := make([]string, 0)
	if host == "" {
		missing = append(missing, "DB_HOST")
	}
	if user == "" {
		missing = append(missing, "DB_USER")
	}
	if password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if dbName == "" {
		missing = append(missing, "DB_NAME")
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("database config missing: set DB_URL or DB_HOST/DB_USER/DB_PASSWORD/DB_NAME (missing: %s)", strings.Join(missing, ", "))
	}

	if port == "" {
		port = "5432"
	}

	sslMode := strings.TrimSpace(os.Getenv("DB_SSLMODE"))
	if sslMode == "" {
		sslMode = defaultSSLModeForHost(host)
	}

	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   dbName,
	}

	q := dsn.Query()
	q.Set("sslmode", sslMode)
	dsn.RawQuery = q.Encode()

	built := dsn.String()
	_ = os.Setenv("DB_URL", built)
	return built, nil
}

func defaultSSLModeForHost(host string) string {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1", "postgres":
		return "disable"
	default:
		return "require"
	}
}
