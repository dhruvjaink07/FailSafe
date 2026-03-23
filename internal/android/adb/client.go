package adb

// Wrapper around adb client to execute commands on android devices. This will be used by the fault injector and monitor to interact with the device.

type Client struct {
	deviceID string
	adbPath  string
}

func NewClient(deviceID string, adbPath string) *Client

func (c *Client) Shell(command string) (string, error)

func (c *Client) Exec(args ...string) (string, error)
