package ssm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/system"
)

var (
	_             daemon.Daemon = &ssm{}
	SsmDaemonName               = "amazon-ssm-agent"

	checksumMismatchErrorRegex = regexp.MustCompile(`.*checksum mismatch with latest ssm-setup-cli*`)
	activationExpiredRegex     = regexp.MustCompile(`.*ActivationExpired*`)
	invalidActivationRegex     = regexp.MustCompile(`.*InvalidActivation*`)
	defaultAWSConfigPath       = "/root/.aws"
	eksHybridPath              = "/eks-hybrid"
	symlinkedAWSConfigPath     = filepath.Join(eksHybridPath, ".aws")
)

type ssm struct {
	daemonManager daemon.DaemonManager
	nodeConfig    *api.NodeConfig
	logger        *zap.Logger
}

func NewSsmDaemon(daemonManager daemon.DaemonManager, cfg *api.NodeConfig, logger *zap.Logger) daemon.Daemon {
	setDaemonName()
	return &ssm{
		daemonManager: daemonManager,
		nodeConfig:    cfg,
		logger:        logger,
	}
}

func (s *ssm) Configure() error {
	registerOverride := false
	_, err := GetManagedHybridInstanceId()
	if err != nil && os.IsNotExist(err) {
		// The node is not registered with SSM
		// In some cases, while the node is not registered, there might be some leftover
		// registration data from previous registrations. Setting force to true, will override
		// leftover registration data from the service cache.
		registerOverride = true
	} else if err != nil {
		return err
	}
	err = s.registerMachine(s.nodeConfig, registerOverride)
	if err != nil {
		// SSM register command will download a new ssm agent installer and verify checksums to match with
		// downloaded and current running agent installer. If checksums do not match, re-download and run
		// register again. Checksum mismatch can happen due to new ssm agent releases or switching regions.
		if match := checksumMismatchErrorRegex.MatchString(err.Error()); match {
			s.logger.Info("Encountered checksum mismatch on SSM agent installer. Re-downloading installer from",
				zap.Reflect("region", s.nodeConfig.Spec.Cluster.Region))
			if err := redownloadInstaller(s.nodeConfig.Spec.Cluster.Region); err != nil {
				return err
			}
			return s.registerMachine(s.nodeConfig, registerOverride)
		} else if match := activationExpiredRegex.MatchString(err.Error()); match {
			return fmt.Errorf("SSM activation expired. Please use a valid activation")
		} else if match := invalidActivationRegex.MatchString(err.Error()); match {
			return fmt.Errorf("invalid SSM activation. Please use a valid activation code, activation id and region")
		}
		return err
	}
	return nil
}

func (s *ssm) EnsureRunning() error {
	err := s.daemonManager.EnableDaemon(SsmDaemonName)
	if err != nil {
		return err
	}
	return s.daemonManager.StartDaemon(SsmDaemonName)
}

func (s *ssm) PostLaunch() error {
	if s.nodeConfig.Spec.Hybrid.EnableCredentialsFile {
		s.logger.Info("Creating symlink for AWS credentials", zap.String("Symbolic link path", symlinkedAWSConfigPath))
		err := os.MkdirAll(eksHybridPath, 0755)
		if err != nil {
			return fmt.Errorf("creating path: %v", err)
		}

		err = os.RemoveAll(symlinkedAWSConfigPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing directory %s: %v", symlinkedAWSConfigPath, err)
		}

		err = os.Symlink(defaultAWSConfigPath, symlinkedAWSConfigPath)
		if err != nil {
			return fmt.Errorf("creating symlink: %v", err)
		}
	}

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
