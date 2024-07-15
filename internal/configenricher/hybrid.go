package configenricher

import (
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/aws/ecr"

	"go.uber.org/zap"
)

type hybridConfigEnricher struct {
	logger *zap.Logger
}

func newHybridConfigEnricher(logger *zap.Logger) ConfigEnricher {
	return &hybridConfigEnricher{
		logger: logger,
	}
}

func (hce *hybridConfigEnricher) Enrich(cfg *api.NodeConfig) error {
	eksRegistry, err := ecr.GetEKSHybridRegistry(cfg.Spec.Cluster.Region)
	if err != nil {
		return err
	}
	cfg.Status.Defaults = api.DefaultOptions{
		SandboxImage: eksRegistry.GetSandboxImage(),
	}
	hce.logger.Info("Default options populated", zap.Reflect("defaults", cfg.Status.Defaults))
	return nil
}
