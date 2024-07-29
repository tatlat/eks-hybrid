package containerd

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
)

const ContainerdDaemonName = "containerd"

var _ daemon.Daemon = &containerd{}

type containerd struct {
	daemonManager daemon.DaemonManager
	nodeConfig    *api.NodeConfig
	awsConfig     *aws.Config
}

func NewContainerdDaemon(daemonManager daemon.DaemonManager, cfg *api.NodeConfig, awsConfig *aws.Config) daemon.Daemon {
	return &containerd{
		daemonManager: daemonManager,
		nodeConfig:    cfg,
		awsConfig:     awsConfig,
	}
}

func (cd *containerd) Configure() error {
	if err := writeContainerdConfig(cd.nodeConfig); err != nil {
		return err
	}
	return writeContainerdKernelModulesConfig(cd.daemonManager)
}

// EnsureRunning ensures containerd is running with the written configuration
// With some installations, containerd daemon is already in an running state
// This enables the daemon and restarts or starts depending on the state of daemon
func (cd *containerd) EnsureRunning() error {
	err := cd.daemonManager.EnableDaemon(ContainerdDaemonName)
	if err != nil {
		return err
	}
	return cd.daemonManager.RestartDaemon(ContainerdDaemonName)
}

func (cd *containerd) PostLaunch() error {
	return cacheSandboxImage(cd.awsConfig)
}

func (cd *containerd) Stop() error {
	return cd.daemonManager.StopDaemon(ContainerdDaemonName)
}

func (cd *containerd) Name() string {
	return ContainerdDaemonName
}
