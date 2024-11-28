//go:build !linux

package daemon

import "context"

var _ DaemonManager = &noopDaemonManager{}

type noopDaemonManager struct{}

func NewDaemonManager() (DaemonManager, error) {
	return &noopDaemonManager{}, nil
}

func (m *noopDaemonManager) StartDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) StopDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) RestartDaemon(ctx context.Context, name string, opts ...OperationOption) error {
	return nil
}

func (m *noopDaemonManager) GetDaemonStatus(name string) (DaemonStatus, error) {
	return DaemonStatusUnknown, nil
}

func (m *noopDaemonManager) EnableDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) DisableDaemon(name string) error {
	return nil
}

func (m *noopDaemonManager) DaemonReload() error {
	return nil
}

func (m *noopDaemonManager) Close() {}
