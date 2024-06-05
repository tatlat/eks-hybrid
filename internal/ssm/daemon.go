package ssm

import (
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/util"
)

var (
	_             daemon.Daemon = &ssm{}
	SsmDaemonName               = "amazon-ssm-agent"
)

type ssm struct {
	daemonManager daemon.DaemonManager
}

func NewSsmDaemon(daemonManager daemon.DaemonManager) daemon.Daemon {
	setDaemonName()
	return &ssm{daemonManager: daemonManager}
}

func (s *ssm) Configure(cfg *api.NodeConfig) error {
	return s.registerMachine(cfg)
}

func (s *ssm) EnsureRunning() error {
	return s.daemonManager.StartDaemon(SsmDaemonName)
}

func (s *ssm) PostLaunch(_ *api.NodeConfig) error {
	return nil
}

func (s *ssm) Name() string {
	return SsmDaemonName
}

func setDaemonName() {
	osToDaemonName := map[string]string{
		"ubuntu": "snap.amazon-ssm-agent.amazon-ssm-agent",
		"rhel":   "amazon-ssm-agent",
		"amzn":   "amazon-ssm-agent",
	}
	osName := util.GetOsName()
	if daemonName, ok := osToDaemonName[osName]; ok {
		SsmDaemonName = daemonName
	}
}
