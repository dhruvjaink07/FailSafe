package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func runAAPT(apkPath string) (string, error) {
	if _, err := os.Stat(apkPath); err != nil {
		return "", fmt.Errorf("apk file not found: %s", apkPath)
	}

	cmd := exec.Command("aapt", "dump", "badging", apkPath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("aapt failed: %v", err)
	}

	return string(out), nil
}

func parsePackageFromAAAPT(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "package:") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "name='") {
					pkg := strings.TrimPrefix(part, "name='")
					pkg = strings.TrimSuffix(pkg, "'")
					return pkg
				}
			}
		}
	}
	return ""
}

func parseActivityFromAAAPT(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "launchable-activity:") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "name='") {
					activity := strings.TrimPrefix(part, "name='")
					activity = strings.TrimSuffix(activity, "'")
					return activity
				}
			}
		}
	}
	return ""
}

type AAAPTResponse struct {
	Package  string `json:"package"`
	Activity string `json:"activity"`
	Output   string `json:"output"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

func handleAAAPT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apkPath := r.URL.Query().Get("apk")
	if strings.TrimSpace(apkPath) == "" {
		http.Error(w, "apk query parameter required", http.StatusBadRequest)
		return
	}

	output, err := runAAPT(apkPath)
	resp := AAAPTResponse{
		Output:  output,
		Success: err == nil,
	}

	if err != nil {
		resp.Error = err.Error()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp.Package = parsePackageFromAAAPT(output)
	resp.Activity = parseActivityFromAAAPT(output)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/aapt", handleAAAPT)

	addr := ":9090"
	log.Printf("Android executor listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
