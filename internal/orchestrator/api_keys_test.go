package orchestrator

import (
	"strings"
	"testing"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func TestStartExperimentBlocksHighIntensityInProd(t *testing.T) {
	o := NewOrchestrator(nil, "", "", "", "", "")
	apiCtx := &models.APIContext{KeyID: "key-1", Env: "prod", Role: "engineer"}

	_, err := o.StartExperiment(
		"network_delay",
		[]string{"svc"},
		string(models.TargetDocker),
		nil,
		"http",
		20,
		false,
		1,
		61,
		nil,
		nil,
		nil,
		models.ExpectedState{},
		nil,
		nil,
		nil,
		apiCtx,
	)

	if err == nil || !strings.Contains(err.Error(), "intensity too high for prod") {
		t.Fatalf("expected prod intensity block, got %v", err)
	}
}
