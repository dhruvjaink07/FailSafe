package emulator

// start/stop emulator

type Manager struct{}

func (m *Manager) Start(avdName string) error

func (m *Manager) WaitForBoot(device string) error

func (m *Manager) Stop(device string) error
