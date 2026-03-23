package fault

import (
	"errors"

	"github.com/dhruvjaink07/failsafe/internal/android/adb"
)

// AndroidInjector implements Injector interface for Android targets
type AndroidInjector struct {
	adb *adb.Client
	pkg string // app package name
}

func NewAndroidInjector(adbCliet *adb.Client, pkg string) *AndroidInjector {
	return &AndroidInjector{
		adb: adbCliet,
		pkg: pkg,
	}
}

//
// ---------------- FAULT TYPES ----------------
//

const (
	FaultKillApp        FaultType = "kill_app"
	FaultNetworkDisable FaultType = "network_disable"
	FaultNetworkEnable  FaultType = "network_enable"
	FaultClearData      FaultType = "clear_data"
)

//
// ---------------- INJECTOR IMPLEMENTATION ----------------
//

// Inject executes a fault based on config
func (i *AndroidInjector) Inject(config FaultConfig) error {

	switch config.Type {
	// kill the app process (simulate crash / OS kill)
	case FaultKillApp:
		return i.adb.ForceStop(i.pkg)

	// disable network (simulate connectivity loss)
	case FaultNetworkDisable:
		if err := i.adb.DisableWifi(); err != nil {
			return err
		}
		return i.adb.DisableData()

	// Enable network (restore state)
	case FaultNetworkEnable:
		if err := i.adb.EnableWifi(); err != nil {
			return err
		}
		return i.adb.EnableData()

	// Clear app data (simulate fresh install state)
	case FaultClearData:
		return i.adb.ClearData(i.pkg)

	default:
		return errors.New("unknown fault type")
	}
}
