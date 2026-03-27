package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
	"github.com/dhruvjaink07/failsafe/internal/storage"
)

func main() {

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
	Targets           []string `json:"targets"`
	TargetType        string   `json:"targetType"`
	ObservedEndpoints []string `json:"observedEndpoints"`
	ObservationType   string   `json:"observationType"`
	Duration          int      `json:"duration"`

	Adaptive        bool                `json:"adaptive"`
	StepIntensity   int                 `json:"stepIntensity"`
	MaxIntensity    int                 `json:"maxIntensity"`
	DependencyGraph map[string][]string `json:"dependencyGraph"`

	TargetEndpointMap map[string][]string     `json:"targetEndpointMap"`
	Scenario          []models.ScheduledFault `json:"scenario"`
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
		log.Println("JSON decode error:", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.FaultType == "" || len(req.Targets) == 0 {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	exp, err := orch.StartExperiment(
		req.FaultType,
		req.Targets,
		req.TargetType,
		req.ObservedEndpoints,
		req.ObservationType,
		req.Duration,
		req.Adaptive,
		req.StepIntensity,
		req.MaxIntensity,
		req.DependencyGraph,
		req.TargetEndpointMap,
		req.Scenario,
	)

	log.Printf("REQ: %+v\n", req)
	if err != nil {
		log.Println("StartExperiment error:", err)
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
