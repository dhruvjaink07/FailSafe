package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
)

func main() {

	orch := orchestrator.NewOrchestrator()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/experiment/start", func(w http.ResponseWriter, r *http.Request) {
		startHandler(w, r, orch)
	})
	http.HandleFunc("/experiment/get", func(w http.ResponseWriter, r *http.Request) {
		getHandler(w, r, orch)
	})
	http.HandleFunc("/experiment/stop", func(w http.ResponseWriter, r *http.Request) {
		stopHandler(w, r, orch)
	})
	http.HandleFunc("/experiment/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricsHandler(w, r, orch)
	})

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

/*
---------------------------------------
Request Models
---------------------------------------
*/

type StartRequest struct {
	FaultType         string   `json:"faultType"`
	TargetContainers  []string `json:"targetContainers"`
	ObservedEndpoints []string `json:"observedEndpoints"`
	Duration          int      `json:"duration"`

	Adaptive        bool                `json:"adaptive"`
	StepIntensity   int                 `json:"stepIntensity"`
	MaxIntensity    int                 `json:"maxIntensity"`
	DependencyGraph map[string][]string `json:"dependencyGraph"`
}

/*
---------------------------------------
Handlers
---------------------------------------
*/

func startHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	exp, err := orch.StartExperiment(
		req.FaultType,
		req.TargetContainers,
		req.ObservedEndpoints,
		req.Duration,
		req.Adaptive,
		req.StepIntensity,
		req.MaxIntensity,
		req.DependencyGraph,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(exp)
}

func getHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	exp, err := orch.GetExperiment(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(exp)
}

func stopHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	if err := orch.StopExperiment(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write([]byte("experiment stopped"))
}

func metricsHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	data, err := orch.GetMetrics(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(data)
}
