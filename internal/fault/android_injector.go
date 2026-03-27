package fault

import (
	"errors"
	"time"

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

//
// ---------------- INJECTOR IMPLEMENTATION ----------------
//

// Inject executes a fault based on config
func (i *AndroidInjector) Inject(config FaultConfig) error {

	switch config.Type {
	// kill the app process (simulate crash / OS kill)
	case FaultKillApp:
		return i.adb.ForceStop(i.pkg)

	// repeatedly kill process to expose restart resilience.
	case FaultKillRepeated:
		repeats := 3
		if config.DurationSeconds > 0 {
			repeats = config.DurationSeconds
		}
		for n := 0; n < repeats; n++ {
			if err := i.adb.ForceStop(i.pkg); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
		}
		return nil

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

	// brief runtime network interruption and recovery
	case FaultNetworkFlaky:
		if err := i.adb.DisableWifi(); err != nil {
			return err
		}
		if err := i.adb.DisableData(); err != nil {
			return err
		}

		waitSeconds := 3
		if config.DurationSeconds > 0 {
			waitSeconds = config.DurationSeconds
		}
		time.Sleep(time.Duration(waitSeconds) * time.Second)

		if err := i.adb.EnableWifi(); err != nil {
			return err
		}
		return i.adb.EnableData()

	// approximate latency by cycling connectivity briefly.
	case FaultNetworkLatency, FaultNetworkLoss:
		if err := i.adb.DisableWifi(); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		return i.adb.EnableWifi()

	case FaultRevokeCamera:
		return i.adb.RevokePermission(i.pkg, "android.permission.CAMERA")

	case FaultRevokeStorage:
		if err := i.adb.RevokePermission(i.pkg, "android.permission.READ_EXTERNAL_STORAGE"); err != nil {
			return err
		}
		return i.adb.RevokePermission(i.pkg, "android.permission.WRITE_EXTERNAL_STORAGE")

	case FaultRevokeLocation:
		if err := i.adb.RevokePermission(i.pkg, "android.permission.ACCESS_FINE_LOCATION"); err != nil {
			return err
		}
		return i.adb.RevokePermission(i.pkg, "android.permission.ACCESS_COARSE_LOCATION")

	case FaultBackgroundApp:
		return i.adb.SendHome()

	case FaultForegroundApp:
		return i.adb.BringToForeground(i.pkg)

	// Clear app data (simulate fresh install state)
	case FaultClearData:
		return i.adb.ClearData(i.pkg)

	default:
		return errors.New("unknown fault type")
	}
}
