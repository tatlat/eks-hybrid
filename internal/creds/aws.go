package creds

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
)

const iamRoleAnywhereProfileName = "hybrid"

func ReadConfig(ctx context.Context, node *api.NodeConfig, opts ...func(*config.LoadOptions) error) (aws.Config, error) {
	if !node.IsHybridNode() {
		if node.Spec.Cluster.Region != "" {
			opts = append(opts, config.WithRegion(node.Spec.Cluster.Region))
		}
		return config.LoadDefaultConfig(ctx, opts...)
	}
	if node.IsSSM() {
		opts = append(opts, config.WithRegion(node.Spec.Cluster.Region))
		return config.LoadDefaultConfig(ctx, opts...)
	}

	if node.IsIAMRolesAnywhere() {
		awsConfigPath := node.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath

		if awsConfigPath == "" {
			awsConfigPath = iamrolesanywhere.DefaultAWSConfigPath
		}

		opts = append(opts,
			config.WithRegion(node.Spec.Cluster.Region),
			config.WithSharedConfigFiles([]string{awsConfigPath}),
			config.WithSharedConfigProfile(iamRoleAnywhereProfileName),
		)

		return config.LoadDefaultConfig(ctx, opts...)
	}

	return aws.Config{}, errors.New("don't know how to build aws config for node config: only EC2, SSM or IAM Roles Anywhere are supported")
}
