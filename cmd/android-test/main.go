package main

import (
	"fmt"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/dhruvjaink07/failsafe/internal/orchestrator"
	"github.com/dhruvjaink07/failsafe/internal/storage"
)

func main() {

	// ---- CONFIG (set your actual values) ----
	adbPath := "C:\\Users\\Lenovo\\AppData\\Local\\Android\\Sdk\\platform-tools\\adb.exe"
	emulatorPath := "C:\\Users\\Lenovo\\AppData\\Local\\Android\\Sdk\\emulator\\emulator.exe"
	apkPath := "D:\\gdg-mumbai-ai-hack-cyber\\code\\build\\app\\outputs\\flutter-apk\\app-release.apk"
	pkg := "com.example.code"
	activity := ".MainActivity"

	// no DB for now
	var db *storage.Postgres = nil

	// ---- INIT ORCHESTRATOR ----
	o := orchestrator.NewOrchestrator(
		db,
		adbPath,
		emulatorPath,
		apkPath,
		pkg,
		activity,
	)

	// ---- RUN EXPERIMENT ----
	expectedRunning := true
	scenario := []models.ScheduledFault{
		{Type: "network_disable", At: 3, DurationSeconds: 6, Trigger: &models.FaultTrigger{Type: "request", Pattern: "/login", TimeoutSeconds: 10}},
		{Type: "network_enable", At: 10, DurationSeconds: 1},
	}

	exp, err := o.StartExperiment(
		"kill_app",    // faultType
		[]string{pkg}, // targets
		"android",     // targetType
		nil,           // observedEndpoints
		"android",     // observationType
		30,            // duration
		false,         // adaptive
		1,
		5,
		nil,
		nil,
		scenario,
		models.ExpectedState{Running: &expectedRunning, NotCrash: true, NotANR: true},
		nil,
		nil,
		nil,
		nil,
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("Experiment started:", exp.ID)

	// ---- WAIT FOR EXECUTION ----
	time.Sleep(40 * time.Second)

	// ---- FETCH RESULT ----
	result, err := o.GetMetrics(exp.ID)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nRESULT:\n%+v\n", result)
}
