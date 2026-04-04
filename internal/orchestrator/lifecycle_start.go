package orchestrator

import (
	"errors"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
	"github.com/dhruvjaink07/failsafe/internal/fault"
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/monitoring"
	"github.com/google/uuid"
)

type AndroidRunOptions struct {
	AVDName       string
	Headless      bool
	ResetAppState bool
	// UI: "background" (default) or "foreground" (show emulator window)
	UIMode string // "background" or "foreground"
}

type AndroidAppConfig struct {
	APKPath  string
	Package  string
	Activity string
}

func (o *Orchestrator) StartExperiment(
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
	androidOptions *AndroidRunOptions,
	androidApp *AndroidAppConfig,
	frontendRun *models.FrontendRunConfig,
) (*models.Experiment, error) {
	if duration <= 0 {
		return nil, errors.New("invalid duration")
	}
	if err := o.requireFrontendRunConfig(targetType, frontendRun); err != nil {
		return nil, err
	}

	id := uuid.New().String()
	resolvedAndroidApp := &AndroidAppConfig{
		APKPath:  o.apkPath,
		Package:  o.pkg,
		Activity: o.activity,
	}
	if androidApp != nil {
		if androidApp.APKPath != "" {
			resolvedAndroidApp.APKPath = androidApp.APKPath
		}
		if androidApp.Package != "" {
			resolvedAndroidApp.Package = androidApp.Package
		}
		if androidApp.Activity != "" {
			resolvedAndroidApp.Activity = androidApp.Activity
		}
	}

	var adbClient *adb.Client
	isAndroid := strings.EqualFold(targetType, string(models.TargetAndroid))
	isFrontendOnly := strings.EqualFold(targetType, string(models.TargetFrontend)) || strings.EqualFold(targetType, "web")
	var useDeps models.DependencyGraph = deps
	if isAndroid {
		if resolvedAndroidApp.APKPath == "" || resolvedAndroidApp.Package == "" || resolvedAndroidApp.Activity == "" {
			return nil, errors.New("android app config missing apk path/package/activity")
		}
		// UI mode: default to background if not set
		uiMode := "background"
		if androidOptions != nil && androidOptions.UIMode != "" {
			uiMode = androidOptions.UIMode
		}
		if androidOptions != nil {
			androidOptions.Headless = (uiMode == "background")
		}
		// For Android, skip dependency graph (individual app testing)
		useDeps = nil
		client, err := o.setupAndroid(id, androidOptions, resolvedAndroidApp)
		if err != nil {
			return nil, err
		}
		adbClient = client
	} else if !isFrontendOnly {
		o.setupDocker(targets)
	}

	exp := o.createExperiment(id, targets, targetType, observationType, faultType, duration, adaptive, stepIntensity, maxIntensity, useDeps, targetMap)
	exp.Scenario = scheduledFaults
	exp.Expected = expected
	exp.FrontendRun = frontendRun
	if isAndroid {
		exp.APKPath = resolvedAndroidApp.APKPath
		exp.Package = resolvedAndroidApp.Package
		exp.Activity = resolvedAndroidApp.Activity
	}
	o.registerExperiment(id, exp, observedEndpoints)

	callback := o.createCallback(id)

	var injector fault.Injector
	var monitor monitoring.MonitorInterface

	if isAndroid {
		injector = fault.NewAndroidInjector(adbClient, resolvedAndroidApp.Package)
		monitor = monitoring.NewAndroidMonitorWithCallback(callback, adbClient, resolvedAndroidApp.Package)
	} else if isFrontendOnly {
		injector = fault.NewWebInjector()
		monitor = monitoring.NewWebMonitor(callback)
	} else {
		injector = fault.NewDockerInjector(o.docker)
		monitor = monitoring.NewMonitor(callback, o.docker, targets)
	}

	o.mu.Lock()
	o.injectors[id] = injector
	o.monitors[id] = monitor
	o.mu.Unlock()

	// Set phase to injecting before starting monitor to ensure firstImpact is recorded
	o.setPhase(id, models.PhaseInjecting)
	monitor.Start(id, observedEndpoints)
	o.startMetricsFlusher(id)
	if isFrontendOnly {
		o.startFrontendRunner(id, exp)
	}
	go o.runTimeline(id)

	return exp, nil
}
