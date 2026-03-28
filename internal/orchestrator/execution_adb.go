package orchestrator

import (
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
	"github.com/dhruvjaink07/failsafe/internal/android/emulator"
)

func (o *Orchestrator) setupAndroid(id string, options *AndroidRunOptions) (*adb.Client, error) {
	adbClient := adb.NewClient("emulator-5554", o.adbPath)

	_ = adbClient.KillServer()
	_ = adbClient.StartServer()

	emu := emulator.NewManager(o.emulatorPath)
	avdName := "Pixel_8a"
	headless := true
	if options != nil {
		if strings.TrimSpace(options.AVDName) != "" {
			avdName = strings.TrimSpace(options.AVDName)
		}
		headless = options.Headless
	}

	_, err := emu.Start(avdName, headless)
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
