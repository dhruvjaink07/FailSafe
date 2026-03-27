package orchestrator

import (
	"github.com/dhruvjaink07/failsafe/internal/android/adb"
	"github.com/dhruvjaink07/failsafe/internal/android/emulator"
)

func (o *Orchestrator) setupAndroid(id string) (*adb.Client, error) {
	adbClient := adb.NewClient("emulator-5554", o.adbPath)

	_ = adbClient.KillServer()
	_ = adbClient.StartServer()

	emu := emulator.NewManager(o.emulatorPath)

	_, err := emu.Start("Pixel_8a")
	if err != nil {
		return nil, err
	}

	emu.WaitForReady(adbClient)

	if err := adbClient.Install(o.apkPath); err != nil {
		return nil, err
	}

	if err := adbClient.Launch(o.pkg, o.activity); err != nil {
		return nil, err
	}

	o.androidClients[id] = adbClient

	return adbClient, nil
}