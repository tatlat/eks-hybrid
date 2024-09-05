package hybrid

import (
	"context"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const iamRoleAnywhereProfileName = "hybrid"

func (hnp *hybridNodeProvider) ConfigureAws() error {
	region := hnp.nodeConfig.Spec.Cluster.Region
	if hnp.nodeConfig.IsSSM() {
		awsConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
		if err != nil {
			return err
		}
		hnp.awsConfig = &awsConfig
	} else {
		if hnp.nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath == "" {
			hnp.nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath = iamrolesanywhere.DefaultAWSConfigPath
		}
		if err := iamrolesanywhere.WriteAWSConfig(iamrolesanywhere.AWSConfig{
			TrustAnchorARN:       hnp.nodeConfig.Spec.Hybrid.IAMRolesAnywhere.TrustAnchorARN,
			ProfileARN:           hnp.nodeConfig.Spec.Hybrid.IAMRolesAnywhere.ProfileARN,
			RoleARN:              hnp.nodeConfig.Spec.Hybrid.IAMRolesAnywhere.RoleARN,
			Region:               hnp.nodeConfig.Spec.Cluster.Region,
			NodeName:             hnp.nodeConfig.Spec.Hybrid.NodeName,
			ConfigPath:           hnp.nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath,
			SigningHelperBinPath: iamrolesanywhere.SigningHelperBinPath,
		}); err != nil {
			return err
		}
		awsConfig, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(region),
			config.WithSharedConfigFiles([]string{hnp.nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath}),
			config.WithSharedConfigProfile(iamRoleAnywhereProfileName))
		if err != nil {
			return err
		}
		hnp.awsConfig = &awsConfig
	}
	return nil
}

func (hnp *hybridNodeProvider) GetConfig() *aws.Config {
	return hnp.awsConfig
}
