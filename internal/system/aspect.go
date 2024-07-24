package system

import "github.com/aws/eks-hybrid/internal/api"

type SystemAspect interface {
	Name() string
	Setup(*api.NodeConfig) error
}

func NewNodeAspects(cfg *api.NodeConfig) []SystemAspect {
	if cfg.IsHybridNode() {
		return newHybridNodeAspects()
	}
	return newEc2NodeAspects()
}

func newEc2NodeAspects() []SystemAspect {
	return []SystemAspect{
		NewLocalDiskAspect(),
		NewNetworkingAspect(),
	}
}

func newHybridNodeAspects() []SystemAspect {
	return []SystemAspect{
		NewSysctlAspect(),
		NewSwapAspect(),
	}
}
