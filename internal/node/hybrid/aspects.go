package hybrid

import "github.com/aws/eks-hybrid/internal/system"

func (hnp *hybridNodeProvider) GetAspects() []system.SystemAspect {
	return []system.SystemAspect{
		system.NewSysctlAspect(hnp.nodeConfig),
		system.NewSwapAspect(hnp.nodeConfig),
	}
}
