package ssm

import (
	"os"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/system"
)

var (
	_             daemon.Daemon = &ssm{}
	SsmDaemonName               = "amazon-ssm-agent"
)

type ssm struct {
	daemonManager daemon.DaemonManager
	nodeConfig    *api.NodeConfig
}

func NewSsmDaemon(daemonManager daemon.DaemonManager, cfg *api.NodeConfig) daemon.Daemon {
	setDaemonName()
	return &ssm{
		daemonManager: daemonManager,
		nodeConfig:    cfg,
	}
}

func (s *ssm) Configure() error {
	_, err := GetManagedHybridInstanceId()
	if err != nil && os.IsNotExist(err) {
		// The node is not registered with SSM
		// In some cases, while the node is not registered, there might be some leftover
		// registration data from previous registrations. Setting force to true, will override
		// leftover registration data from the service cache.
		return s.registerMachine(s.nodeConfig, true)
	} else if err != nil {
		return err
	}
	return s.registerMachine(s.nodeConfig, false)
}

func (s *ssm) EnsureRunning() error {
	err := s.daemonManager.EnableDaemon(SsmDaemonName)
	if err != nil {
		return err
	}
	return s.daemonManager.StartDaemon(SsmDaemonName)
}

func (s *ssm) PostLaunch() error {
	return nil
}

// Stop stops the ssm unit only if it is loaded and running
func (s *ssm) Stop() error {
	return s.daemonManager.StopDaemon(SsmDaemonName)

}

func (s *ssm) Name() string {
	return SsmDaemonName
}

func setDaemonName() {
	osToDaemonName := map[string]string{
		system.UbuntuOsName: "snap.amazon-ssm-agent.amazon-ssm-agent",
		system.RhelOsName:   "amazon-ssm-agent",
		system.AmazonOsName: "amazon-ssm-agent",
	}
	osName := system.GetOsName()
	if daemonName, ok := osToDaemonName[osName]; ok {
		SsmDaemonName = daemonName
	}
}
