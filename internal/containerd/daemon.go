package containerd

import (
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
)

const ContainerdDaemonName = "containerd"

var _ daemon.Daemon = &containerd{}

type containerd struct {
	daemonManager daemon.DaemonManager
}

func NewContainerdDaemon(daemonManager daemon.DaemonManager) daemon.Daemon {
	return &containerd{
		daemonManager: daemonManager,
	}
}

func (cd *containerd) Configure(c *api.NodeConfig) error {
	return writeContainerdConfig(c)
}

func (cd *containerd) EnsureRunning() error {
	err := cd.daemonManager.EnableDaemon(ContainerdDaemonName)
	if err != nil {
		return err
	}
	return cd.daemonManager.StartDaemon(ContainerdDaemonName)
}

func (cd *containerd) PostLaunch(c *api.NodeConfig) error {
	return cacheSandboxImage(c)
}

func (cd *containerd) Name() string {
	return ContainerdDaemonName
}
