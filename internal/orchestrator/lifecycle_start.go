package orchestrator

import (
	"errors"

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
) (*models.Experiment, error) {
	if duration <= 0 {
		return nil, errors.New("invalid duration")
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
	if targetType == "android" {
		if resolvedAndroidApp.APKPath == "" || resolvedAndroidApp.Package == "" || resolvedAndroidApp.Activity == "" {
			return nil, errors.New("android app config missing apk path/package/activity")
		}

		client, err := o.setupAndroid(id, androidOptions, resolvedAndroidApp)
		if err != nil {
			return nil, err
		}
		adbClient = client
	} else {
		o.setupDocker(targets)
	}

	exp := o.createExperiment(id, targets, targetType, observationType, faultType, duration, adaptive, stepIntensity, maxIntensity, deps, targetMap)
	exp.Scenario = scheduledFaults
	exp.Expected = expected
	if targetType == "android" {
		exp.APKPath = resolvedAndroidApp.APKPath
		exp.Package = resolvedAndroidApp.Package
		exp.Activity = resolvedAndroidApp.Activity
	}
	o.registerExperiment(id, exp, observedEndpoints)

	callback := o.createCallback(id)

	var injector fault.Injector
	var monitor monitoring.MonitorInterface

	if targetType == "android" {
		injector = fault.NewAndroidInjector(adbClient, resolvedAndroidApp.Package)
		monitor = monitoring.NewAndroidMonitorWithCallback(callback, adbClient, resolvedAndroidApp.Package)
	} else {
		injector = fault.NewDockerInjector(o.docker)
		monitor = monitoring.NewMonitor(callback, o.docker, targets)
	}

	o.mu.Lock()
	o.injectors[id] = injector
	o.monitors[id] = monitor
	o.mu.Unlock()

	monitor.Start(id, observedEndpoints)
	o.startMetricsFlusher(id)
	go o.runTimeline(id)

	return exp, nil
}
