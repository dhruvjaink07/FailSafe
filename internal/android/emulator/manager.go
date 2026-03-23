package emulator

import (
	"os/exec"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
)

// Manager controll emulator lifecycle (start/stop)
type Manager struct {
	emulatoPath string // full path to emulator binary
}

func NewManager(emulatorPath string) *Manager {
	return &Manager{emulatoPath: emulatorPath}
}

// Start Emulator

// launches an emulator as background process
func (m *Manager) Start(avdName string) (*exec.Cmd, error) {

	// Command:
	// emulator -avd <name> -no-window -no-audio -no-boot-anim
	cmd := exec.Command(
		m.emulatoPath,
		"-avd", avdName,
		"-no-window",    // headles no UI
		"-no-audio",     // reduce resource usage
		"-no-boot-anim", // no boot animation
	)

	//  Start process (non-blocking)
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

// WAIT FOR READINESS
// ensures emulator is usable
func (m *Manager) WaitForReady(adbClient *adb.Client) {
	//1. Wait until device is listed in adb devices
	adbClient.WaitForDevice()

	// Small buffer (ADB registeration != fully stable)
	time.Sleep(2 * time.Second)

	//2. Wait until system is fully booted
	adbClient.WaitForBoot()
}

// STOP Emulator
// kills the emulator process
func (m *Manager) Stop(cmd *exec.Cmd) error {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}
