package orchestrator

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

func (o *Orchestrator) startFrontendRunner(id string, exp *models.Experiment) {
	if exp == nil || exp.FrontendRun == nil {
		return
	}

	baseURL := strings.TrimSpace(exp.FrontendRun.BaseURL)
	if baseURL == "" {
		return
	}

	o.mu.Lock()
	if _, exists := o.frontendRunners[id]; exists {
		o.mu.Unlock()
		return
	}
	o.mu.Unlock()

	nodeBin := "node"
	if runtime.GOOS == "windows" {
		nodeBin = "node.exe"
	}

	scriptPath := filepath.Join("internal", "frontend", "automation", "playwright", "runner.js")
	cmd := exec.Command(nodeBin, scriptPath)

	metricsEndpoint := strings.TrimSpace(exp.FrontendRun.MetricsEndpoint)
	if metricsEndpoint == "" {
		metricsEndpoint = "http://localhost:8000/frontend/metrics"
	}

	env := os.Environ()
	headless := strings.TrimSpace(os.Getenv("FAILSAFE_PW_HEADLESS"))
	if headless == "" {
		headless = "false"
	}
	env = append(env, "EXPERIMENT_ID="+id)
	env = append(env, "BASE_URL="+baseURL)
	env = append(env, "FAILSAFE_FRONTEND_ENDPOINT="+metricsEndpoint)
	env = append(env, "PW_HEADLESS="+headless)
	if len(exp.FrontendRun.TargetURLs) > 0 {
		env = append(env, "FAILSAFE_TARGET_URLS="+strings.Join(exp.FrontendRun.TargetURLs, ","))
	}
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("frontend runner failed to start id=%s: %v", id, err)
		return
	}

	o.mu.Lock()
	delete(o.runnerStops, id)
	o.frontendRunners[id] = cmd
	o.mu.Unlock()

	go func(expID string, runner *exec.Cmd) {
		err := runner.Wait()
		o.mu.Lock()
		intentionalStop := o.runnerStops[expID]
		delete(o.runnerStops, expID)
		delete(o.frontendRunners, expID)
		o.mu.Unlock()
		if err != nil {
			if intentionalStop {
				log.Printf("frontend runner exited after stop id=%s", expID)
				return
			}
			log.Printf("frontend runner exited with error id=%s: %v", expID, err)
			return
		}
		log.Printf("frontend runner exited id=%s", expID)
	}(id, cmd)
}

func (o *Orchestrator) stopFrontendRunner(id string) {
	o.mu.Lock()
	runner, ok := o.frontendRunners[id]
	if !ok || runner == nil || runner.Process == nil {
		o.mu.Unlock()
		return
	}
	o.runnerStops[id] = true
	o.mu.Unlock()

	if err := runner.Process.Kill(); err != nil {
		log.Printf("frontend runner stop failed id=%s: %v", id, err)
		return
	}

	o.mu.Lock()
	delete(o.frontendRunners, id)
	o.mu.Unlock()
	log.Printf("frontend runner stopped id=%s", id)
}

func (o *Orchestrator) requireFrontendRunConfig(targetType string, frontendRun *models.FrontendRunConfig) error {
	isFrontendOnly := strings.EqualFold(targetType, string(models.TargetFrontend)) || strings.EqualFold(targetType, "web")
	if !isFrontendOnly {
		return nil
	}
	if frontendRun == nil || strings.TrimSpace(frontendRun.BaseURL) == "" {
		return fmt.Errorf("frontend_run.base_url is required for target_type=frontend")
	}
	return nil
}
