package configenricher

import (
	"github.com/aws/eks-hybrid/internal/api"

	"go.uber.org/zap"
)

type ConfigEnricher interface {
	Enrich(config *api.NodeConfig) error
}

func New(logger *zap.Logger, cfg *api.NodeConfig) ConfigEnricher {
	if cfg.IsHybridNode() {
		return newHybridConfigEnricher(logger)
	}
	return newImdsConfigEnricher(logger)
}