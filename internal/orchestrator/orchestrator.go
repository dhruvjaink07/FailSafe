package orchestrator

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
	"github.com/dhruvjaink07/failsafe/internal/docker"
	"github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
	"github.com/dhruvjaink07/failsafe/internal/storage"
)

type Orchestrator struct {
	experiments map[string]*models.Experiment
	monitors    map[string]monitoring.MonitorInterface
	metrics     map[string]map[string][]models.MetricSample

	downtime     map[string]time.Time
	totalDown    map[string]time.Duration
	failures     map[string]int
	lastRecovery map[string]time.Duration

	docker *docker.Manager

	injectors      map[string]fault.Injector
	androidClients map[string]*adb.Client

	firstImpact  map[string]map[string]time.Time
	recoveryAt   map[string]map[string]time.Time
	faultHistory map[string][]FaultEvent

	db           *storage.Postgres
	metricBuffer map[string][]models.MetricSample

	adbPath      string
	emulatorPath string
	apkPath      string
	pkg          string
	activity     string

	emulatorRunning  bool
	emulatorDeviceID string
	emulatorAVDName  string

	frontendMetrics map[string][]models.FrontendMetrics
	frontendRunners map[string]*exec.Cmd
	runnerStops     map[string]bool

	androidReady      bool
	androidReadyError string

	mu sync.Mutex
}

func NewOrchestrator(
	db *storage.Postgres,
	adbPath, emulatorPath, apkPath, pkg, activity string,
) *Orchestrator {

	orch := &Orchestrator{
		experiments:    make(map[string]*models.Experiment),
		monitors:       make(map[string]monitoring.MonitorInterface),
		metrics:        make(map[string]map[string][]models.MetricSample),
		downtime:       make(map[string]time.Time),
		totalDown:      make(map[string]time.Duration),
		failures:       make(map[string]int),
		lastRecovery:   make(map[string]time.Duration),
		firstImpact:    make(map[string]map[string]time.Time),
		recoveryAt:     make(map[string]map[string]time.Time),
		faultHistory:   make(map[string][]FaultEvent),
		docker:         docker.NewManager(),
		injectors:      make(map[string]fault.Injector),
		androidClients: make(map[string]*adb.Client),
		db:             db,
		metricBuffer:   make(map[string][]models.MetricSample),

		adbPath:      adbPath,
		emulatorPath: emulatorPath,
		apkPath:      apkPath,
		pkg:          pkg,
		activity:     activity,

		emulatorRunning:  false,
		emulatorDeviceID: "emulator-5554",
		emulatorAVDName:  "Pixel_8a",
		frontendMetrics:  make(map[string][]models.FrontendMetrics),
		frontendRunners:  make(map[string]*exec.Cmd),
		runnerStops:      make(map[string]bool),
		androidReady:     true,
	}

	orch.initAndroidPreflight()

	return orch
}

func (o *Orchestrator) GetBackendLogs(apiKeyID, experimentID string, tail int) (string, error) {
	if o.db == nil {
		return "", errors.New("storage not configured")
	}

	owned, err := o.db.IsExperimentOwnedByAPIKey(experimentID, apiKeyID)
	if err != nil {
		return "", err
	}
	if !owned {
		return "", errors.New("experiment not found or access denied")
	}

	exp, err := o.db.GetExperimentSnapshot(experimentID, "")
	if err != nil {
		return "", err
	}
	if exp == nil {
		return "", errors.New("experiment not found")
	}

	if len(exp.Targets) == 0 {
		return "", errors.New("no target containers for experiment")
	}

	// Use first target (container/service name)
	container := exp.Targets[0]
	return o.docker.GetContainerLogs(container, tail)
}

// StreamBackendLogs streams container logs to the provided ResponseWriter.
func (o *Orchestrator) StreamBackendLogs(apiKeyID, experimentID string, tail int, w http.ResponseWriter, format string, follow bool) error {
	if o.db == nil {
		return errors.New("storage not configured")
	}

	owned, err := o.db.IsExperimentOwnedByAPIKey(experimentID, apiKeyID)
	if err != nil {
		return err
	}
	if !owned {
		return errors.New("experiment not found or access denied")
	}

	exp, err := o.db.GetExperimentSnapshot(experimentID, "")
	if err != nil {
		return err
	}
	if exp == nil {
		return errors.New("experiment not found")
	}

	if len(exp.Targets) == 0 {
		return errors.New("no target containers for experiment")
	}

	container := exp.Targets[0]

	// Ask docker manager to start the logs command and return a ReadCloser
	rc, cmd, err := o.docker.StreamContainerLogs(container, tail, follow)
	if err != nil {
		return err
	}

	// Ensure command killed when done
	defer func() {
		_ = cmd.Process.Kill()
		rc.Close()
	}()

	// For streaming, prefer line-oriented streaming. Support SSE when format=="sse".
	reader := bufio.NewReader(rc)
	flusher, _ := w.(http.Flusher)

	if format == "sse" {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				// Trim newline and send as SSE data
				payload := strings.TrimRight(line, "\r\n")
				_, _ = w.Write([]byte("data: " + payload + "\n\n"))
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
		}
		return nil
	}

	// Plain streaming (text) or json chunks: stream raw lines
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			_, _ = w.Write([]byte(line))
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}
