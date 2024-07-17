package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
)

const iamRoleAnywhereProfileName = "hybrid"

type Config interface {
	GetConfig() (aws.Config, error)
}

func NewConfig(nodeCfg *api.NodeConfig) (Config, error) {
	if nodeCfg.IsHybridNode() {
		if nodeCfg.IsIAMRolesAnywhere() {
			if nodeCfg.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath == "" {
				nodeCfg.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath = iamrolesanywhere.DefaultAWSConfigPath
			}
			if err := iamrolesanywhere.EnsureAWSConfig(iamrolesanywhere.AWSConfig{
				TrustAnchorARN: nodeCfg.Spec.Hybrid.IAMRolesAnywhere.TrustAnchorARN,
				ProfileARN:     nodeCfg.Spec.Hybrid.IAMRolesAnywhere.ProfileARN,
				RoleARN:        nodeCfg.Spec.Hybrid.IAMRolesAnywhere.RoleARN,
				Region:         nodeCfg.Spec.Cluster.Region,
				ConfigPath:     nodeCfg.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath,
			}); err != nil {
				return nil, err
			}
			return newAdvancedConfig(nodeCfg.Spec.Cluster.Region,
				nodeCfg.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath,
				iamRoleAnywhereProfileName,
			), nil
		}
		// SSM add comments
		return newDefaultConfig(nodeCfg.Spec.Cluster.Region), nil
	}
	// default add comments
	return newDefaultConfig(nodeCfg.Status.Instance.Region), nil
}
