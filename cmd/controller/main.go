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
	log.Println("Server running on : 8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Orchestrator Handlers
type StartRequest struct {
	FaultType string `json:"faultType"`
	Target    string `json:"target"`
	TargetURL string `json:"targetUrl"`
	Duration  int    `json:"duration"`
}

func startHandler(w http.ResponseWriter, r *http.Request, orch *orchestrator.Orchestrator) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	exp, err := orch.StartExperiment(req.FaultType, req.Target, req.TargetURL, req.Duration)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	err := orch.StopExperiment(id)
	if err != nil {
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
