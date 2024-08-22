package hybrid

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/nodeprovider"
)

type hybridNodeProvider struct {
	nodeConfig    *api.NodeConfig
	validator     func(config *api.NodeConfig) error
	awsConfig     *aws.Config
	daemonManager daemon.DaemonManager
	logger        *zap.Logger
}

func NewHybridNodeProvider(nodeConfig *api.NodeConfig, logger *zap.Logger) (nodeprovider.NodeProvider, error) {
	np := &hybridNodeProvider{
		nodeConfig: nodeConfig,
		logger:     logger,
	}
	np.withHybridValidators()
	if err := np.withDaemonManager(); err != nil {
		return nil, err
	}
	return np, nil
}

func (hnp *hybridNodeProvider) GetNodeConfig() *api.NodeConfig {
	return hnp.nodeConfig
}

func (hnp *hybridNodeProvider) Logger() *zap.Logger {
	return hnp.logger
}

func (hnp *hybridNodeProvider) Cleanup() {
	hnp.daemonManager.Close()
}
