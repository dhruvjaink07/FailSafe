package orchestrator

import (
	"fmt"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
)

func (o *Orchestrator) initAndroidPreflight() {
	adbPath, err := resolveExecutablePath(o.adbPath, "adb")
	if err != nil {
		o.markAndroidUnavailable(fmt.Sprintf("android preflight failed at startup: adb unavailable (%v)", err))
		return
	}
	emulatorPath, err := resolveExecutablePath(o.emulatorPath, "emulator")
	if err != nil {
		o.markAndroidUnavailable(fmt.Sprintf("android preflight failed at startup: emulator unavailable (%v)", err))
		return
	}

	deviceID := o.emulatorDeviceID
	if strings.TrimSpace(deviceID) == "" {
		deviceID = "emulator-5554"
	}
	adbClient := adb.NewClient(deviceID, adbPath)
	if err := adbClient.StartServer(); err != nil {
		o.markAndroidUnavailable(fmt.Sprintf("android preflight failed at startup: unable to start adb server (%v)", err))
		return
	}

	o.mu.Lock()
	o.adbPath = adbPath
	o.emulatorPath = emulatorPath
	o.androidReady = true
	o.androidReadyError = ""
	o.mu.Unlock()
}

func (o *Orchestrator) markAndroidUnavailable(reason string) {
	o.mu.Lock()
	o.androidReady = false
	o.androidReadyError = reason
	o.mu.Unlock()
}

func (o *Orchestrator) androidStartupReadinessError() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.androidReady {
		return ""
	}
	if strings.TrimSpace(o.androidReadyError) == "" {
		return "android preflight failed at startup"
	}
	return o.androidReadyError
}
