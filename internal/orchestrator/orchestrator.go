package orchestrator

import (
	"errors"
	"os/exec"
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

func (o *Orchestrator) GetLatestSystemMetrics() (map[string]interface{}, error) {
	if o.db == nil {
		return nil, errors.New("storage not configured")
	}
	return o.db.GetLatestSystemMetrics()
}

func (o *Orchestrator) ListExperiments() ([]*models.Experiment, error) {
	if o.db == nil {
		return nil, errors.New("storage not configured")
	}
	return o.db.ListExperiments()
}
