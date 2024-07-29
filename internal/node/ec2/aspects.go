package ec2

import "github.com/aws/eks-hybrid/internal/system"

func (enp *ec2NodeProvider) GetAspects() []system.SystemAspect {
	return []system.SystemAspect{
		system.NewLocalDiskAspect(enp.nodeConfig),
		system.NewNetworkingAspect(enp.nodeConfig),
	}
}
