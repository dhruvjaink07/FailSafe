package monitoring

import "github.com/dhruvjaink07/failsafe/internal/android/adb"

type AndroidMonitor struct {
	adb *adb.Client
	pkg string
}
