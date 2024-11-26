package daemon_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aws/eks-hybrid/internal/daemon"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

func TestWaitForStatusSuccess(t *testing.T) {
	ctx := context.Background()
	g := NewWithT(t)
	manager := &fakeManager{}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(5 * time.Millisecond)
		manager.SetStatus(daemon.DaemonStatusRunning)
	}()

	g.Expect(
		daemon.WaitForStatus(ctx, zap.NewNop(), manager, "my-daemon", daemon.DaemonStatusRunning, time.Millisecond),
	).To(Succeed())
}

func TestWaitForStatusTimeout(t *testing.T) {
	ctx := context.Background()
	g := NewWithT(t)
	manager := &fakeManager{}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	g.Expect(
		daemon.WaitForStatus(ctx, zap.NewNop(), manager, "my-daemon", daemon.DaemonStatusRunning, 10*time.Nanosecond),
	).NotTo(MatchError((ContainSubstring("daemon my-daemon still has status unknown: context deadline exceeded"))))
}

type fakeManager struct {
	sync.RWMutex
	status daemon.DaemonStatus
}

var _ daemon.DaemonManager = &fakeManager{}

func (f *fakeManager) RestartDaemon(name string) error {
	return nil
}

func (f *fakeManager) EnableDaemon(name string) error {
	return nil
}

func (f *fakeManager) StopDaemon(name string) error {
	return nil
}

func (f *fakeManager) StartDaemon(name string) error {
	return nil
}

func (f *fakeManager) GetDaemonStatus(name string) (daemon.DaemonStatus, error) {
	f.RLock()
	defer f.RUnlock()
	return f.status, nil
}

func (f *fakeManager) DisableDaemon(name string) error {
	return nil
}

func (f *fakeManager) DaemonReload() error {
	return nil
}

func (f *fakeManager) Close() {
}

func (f *fakeManager) SetStatus(s daemon.DaemonStatus) {
	f.Lock()
	defer f.Unlock()
	f.status = s
}
