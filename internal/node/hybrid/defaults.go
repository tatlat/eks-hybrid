package hybrid

import (
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
)

func (hnp *HybridNodeProvider) PopulateNodeConfigDefaults() {
	PopulateNodeConfigDefaults(hnp.nodeConfig)
}

func PopulateNodeConfigDefaults(nodeConfig *api.NodeConfig) {
	if nodeConfig.IsIAMRolesAnywhere() {
		nodeConfig.Status.Hybrid.NodeName = nodeConfig.Spec.Hybrid.IAMRolesAnywhere.NodeName
		if nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath == "" {
			nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath = iamrolesanywhere.DefaultAWSConfigPath
		}
	}
}
