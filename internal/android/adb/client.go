package adb

import (
	"os/exec"
	"strings"
	"time"
)

// Client represents a single ADB connection to a specific device/emulator
type Client struct {
	deviceID string // e.g. "emulator-5554"
	adbPath  string // full path to adb binary
}

// Constructor
func NewClient(deviceID string, adbPath string) *Client {
	return &Client{
		deviceID: deviceID,
		adbPath:  adbPath,
	}
}

//
// ---------------- CORE EXECUTION ----------------
//

// Exec runs a raw adb command
// Example: adb devices
func (c *Client) Exec(args ...string) (string, error) {
	cmd := exec.Command(c.adbPath, args...)

	// CombinedOutput captures stdout + stderr
	out, err := cmd.CombinedOutput()

	return string(out), err
}

// execWithDevice ensures command runs on specific device
// Example: adb -s emulator-5554 <command>
func (c *Client) execWithDevice(args ...string) (string, error) {
	fullArgs := append([]string{"-s", c.deviceID}, args...)
	return c.Exec(fullArgs...)
}

// Shell runs a command inside Android system
// Example: adb -s emulator shell "getprop"
func (c *Client) Shell(command string) (string, error) {
	return c.execWithDevice("shell", command)
}

//
// ---------------- ADB LIFECYCLE ----------------
//

// Start ADB server
func (c *Client) StartServer() error {
	_, err := c.Exec("start-server")
	return err
}

// Kill ADB server
func (c *Client) KillServer() error {
	_, err := c.Exec("kill-server")
	return err
}

//
// ---------------- DEVICE MANAGEMENT ----------------
//

// List connected devices
func (c *Client) Devices() (string, error) {
	return c.Exec("devices")
}

// Wait until emulator/device appears in adb list
func (c *Client) WaitForDevice() {
	for {
		out, _ := c.Devices()

		// simple check: deviceID present
		if strings.Contains(out, c.deviceID) {
			return
		}

		time.Sleep(1 * time.Second)
	}
}

// Wait until Android OS is fully booted
func (c *Client) WaitForBoot() {
	for {
		out, _ := c.GetProp("sys.boot_completed")

		if strings.TrimSpace(out) == "1" {
			return
		}

		time.Sleep(2 * time.Second)
	}
}

//
// ---------------- APP CONTROL ----------------
//

// Install APK on device
func (c *Client) Install(apkPath string) error {
	_, err := c.execWithDevice("install", "-r", apkPath)
	return err
}

// Launch app activity
// pkg: com.example.app
// activity: .MainActivity
func (c *Client) Launch(pkg, activity string) error {
	cmd := "am start -n " + pkg + "/" + activity
	_, err := c.Shell(cmd)
	return err
}

// Force stop app (simulate crash/kill)
func (c *Client) ForceStop(pkg string) error {
	_, err := c.Shell("am force-stop " + pkg)
	return err
}

// Clear app data (fresh state)
func (c *Client) ClearData(pkg string) error {
	_, err := c.Shell("pm clear " + pkg)
	return err
}

//
// ---------------- NETWORK CONTROL ----------------
//

// Disable WiFi
func (c *Client) DisableWifi() error {
	_, err := c.Shell("svc wifi disable")
	return err
}

// Enable WiFi
func (c *Client) EnableWifi() error {
	_, err := c.Shell("svc wifi enable")
	return err
}

// Disable mobile data
func (c *Client) DisableData() error {
	_, err := c.Shell("svc data disable")
	return err
}

// Enable mobile data
func (c *Client) EnableData() error {
	_, err := c.Shell("svc data enable")
	return err
}

//
// ---------------- SYSTEM / DEBUG ----------------
//

// Get system property (used for boot check)
func (c *Client) GetProp(key string) (string, error) {
	return c.Shell("getprop " + key)
}

// Get full logs (used for crash detection)
func (c *Client) Logcat() (string, error) {
	return c.execWithDevice("logcat", "-d")
}

// Get system diagnostics
// Example: memory, activity, cpu
func (c *Client) Dumpsys(service string) (string, error) {
	return c.Shell("dumpsys " + service)
}
