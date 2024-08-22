package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/nodeprovider"
	"go.uber.org/zap"
)

type ec2NodeProvider struct {
	nodeConfig    *api.NodeConfig
	awsConfig     *aws.Config
	daemonManager daemon.DaemonManager
	logger        *zap.Logger
	validator     func(config *api.NodeConfig) error
}

func NewEc2NodeProvider(nodeConfig *api.NodeConfig, logger *zap.Logger) (nodeprovider.NodeProvider, error) {
	np := &ec2NodeProvider{
		nodeConfig: nodeConfig,
		logger:     logger,
	}
	np.withEc2NodeValidators()
	if err := np.withDaemonManager(); err != nil {
		return nil, err
	}
	return np, nil
}

func (enp *ec2NodeProvider) GetNodeConfig() *api.NodeConfig {
	return enp.nodeConfig
}

func (enp *ec2NodeProvider) Logger() *zap.Logger {
	return enp.logger
}

func (enp *ec2NodeProvider) Cleanup() {
	enp.daemonManager.Close()
}
