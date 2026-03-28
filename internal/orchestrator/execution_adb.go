package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
	"github.com/dhruvjaink07/failsafe/internal/android/emulator"
)

func (o *Orchestrator) setupAndroid(id string, options *AndroidRunOptions, app *AndroidAppConfig) (*adb.Client, error) {
	if app == nil {
		app = &AndroidAppConfig{APKPath: o.apkPath, Package: o.pkg, Activity: o.activity}
	}

	deviceID := o.emulatorDeviceID
	if strings.TrimSpace(deviceID) == "" {
		deviceID = "emulator-5554"
	}

	adbClient := adb.NewClient(deviceID, o.adbPath)

	_ = adbClient.StartServer()

	avdName := o.emulatorAVDName
	if strings.TrimSpace(avdName) == "" {
		avdName = "Pixel_8a"
	}
	headless := true
	if options != nil {
		if strings.TrimSpace(options.AVDName) != "" {
			avdName = strings.TrimSpace(options.AVDName)
		}
		headless = options.Headless
	}

	if err := o.ensureEmulatorReady(adbClient, avdName, headless); err != nil {
		return nil, err
	}

	if options != nil && options.ResetAppState && app.Package != "" {
		_ = adbClient.ClearData(app.Package)
	}

	if err := adbClient.Install(app.APKPath); err != nil {
		return nil, err
	}

	if err := adbClient.Launch(app.Package, app.Activity); err != nil {
		return nil, err
	}

	o.androidClients[id] = adbClient

	return adbClient, nil
}

func (o *Orchestrator) ensureEmulatorReady(adbClient *adb.Client, avdName string, headless bool) error {
	if o.isConnectedAndBooted(adbClient) {
		o.mu.Lock()
		o.emulatorRunning = true
		o.emulatorAVDName = avdName
		o.mu.Unlock()
		return nil
	}

	if err := o.startAndWaitEmulator(adbClient, avdName, headless, 180*time.Second); err != nil {
		_ = o.killEmulator(adbClient)
		if retryErr := o.startAndWaitEmulator(adbClient, avdName, headless, 180*time.Second); retryErr != nil {
			return fmt.Errorf("failed to start emulator after retry: %w", retryErr)
		}
	}

	o.mu.Lock()
	o.emulatorRunning = true
	o.emulatorAVDName = avdName
	o.mu.Unlock()

	return nil
}

func (o *Orchestrator) startAndWaitEmulator(adbClient *adb.Client, avdName string, headless bool, timeout time.Duration) error {
	emu := emulator.NewManager(o.emulatorPath)
	if _, err := emu.Start(avdName, headless); err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if o.isConnectedAndBooted(adbClient) {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("emulator boot timeout")
}

func (o *Orchestrator) isConnectedAndBooted(adbClient *adb.Client) bool {
	devicesOut, err := adbClient.Devices()
	if err != nil {
		return false
	}

	deviceID := o.emulatorDeviceID
	if strings.TrimSpace(deviceID) == "" {
		deviceID = "emulator-5554"
	}

	if !strings.Contains(devicesOut, deviceID+"\tdevice") {
		return false
	}

	boot, err := adbClient.GetProp("sys.boot_completed")
	if err != nil {
		return false
	}

	return strings.TrimSpace(boot) == "1"
}

func (o *Orchestrator) killEmulator(adbClient *adb.Client) error {
	deviceID := o.emulatorDeviceID
	if strings.TrimSpace(deviceID) == "" {
		deviceID = "emulator-5554"
	}

	_, err := adbClient.Exec("-s", deviceID, "emu", "kill")
	o.mu.Lock()
	o.emulatorRunning = false
	o.mu.Unlock()

	return err
}
