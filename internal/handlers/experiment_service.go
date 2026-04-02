package handlers

import (
	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
)

type ExperimentService interface {
	StartExperiment(
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
	) (*models.Experiment, error)
	GetExperiment(id string) (map[string]interface{}, error)
	StopExperiment(id string) error
	GetBackendMetrics(id string) (interface{}, error)
	GetAndroidMetrics(id string) (interface{}, error)
	GetAndroidStatus(id string) (map[string]interface{}, error)
	GetExperimentTargetType(id string) (string, error)
	GetFrontendMetrics(id string) (interface{}, error)
	GetFrontendFaultCommand(id string) (map[string]interface{}, error)
	AddFrontendMetrics(data []models.FrontendMetrics)
}
