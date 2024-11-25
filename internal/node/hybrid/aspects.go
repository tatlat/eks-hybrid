package hybrid

import "github.com/aws/eks-hybrid/internal/system"

func (hnp *HybridNodeProvider) GetAspects() []system.SystemAspect {
	return []system.SystemAspect{
		system.NewSysctlAspect(hnp.nodeConfig),
		system.NewSwapAspect(hnp.nodeConfig, hnp.logger),
		system.NewPortsAspect(hnp.nodeConfig, hnp.logger),
	}
}
