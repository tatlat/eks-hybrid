package hybrid

import (
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/aws/ecr"
	"go.uber.org/zap"
)

func (hnp *hybridNodeProvider) Enrich() error {
	hnp.logger.Info("Enriching configuration...")
	eksRegistry, err := ecr.GetEKSHybridRegistry(hnp.nodeConfig.Spec.Cluster.Region)
	if err != nil {
		return err
	}
	hnp.nodeConfig.Status.Defaults = api.DefaultOptions{
		SandboxImage: eksRegistry.GetSandboxImage(),
	}
	hnp.logger.Info("Default options populated", zap.Reflect("defaults", hnp.nodeConfig.Status.Defaults))
	return nil
}
