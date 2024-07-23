package kubelet

import (
	"context"
	"encoding/base64"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/internal/api"

	internalaws "github.com/aws/eks-hybrid/internal/aws"
)

func (k *kubelet) ensureClusterDetails(cfg *api.NodeConfig) error {
	if cfg.Spec.Cluster.APIServerEndpoint == "" || cfg.Spec.Cluster.CertificateAuthority == nil || cfg.Spec.Cluster.CIDR == "" {
		awsConfigProvider, err := internalaws.NewConfig(cfg)
		if err != nil {
			return err
		}
		awsConfig, err := awsConfigProvider.GetConfig()
		if err != nil {
			return err
		}
		eksClient := eks.NewFromConfig(awsConfig)
		cluster, err := eksClient.DescribeCluster(context.Background(), &eks.DescribeClusterInput{
			Name: aws.String(cfg.Spec.Cluster.Name),
		})
		if err != nil {
			return err
		}
		if cfg.Spec.Cluster.APIServerEndpoint == "" {
			cfg.Spec.Cluster.APIServerEndpoint = *cluster.Cluster.Endpoint
		}

		if cfg.Spec.Cluster.CertificateAuthority == nil {
			// CertificateAuthority from describeCluster api call returns base64 encoded data as a string
			// Decoding the string to byte array ensures the proper data format when writing to file
			decoded, err := base64.StdEncoding.DecodeString(*cluster.Cluster.CertificateAuthority.Data)
			if err != nil {
				return err
			}
			cfg.Spec.Cluster.CertificateAuthority = decoded
		}

		if cfg.Spec.Cluster.CIDR == "" {
			cfg.Spec.Cluster.CIDR = *cluster.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr
		}
	}
	return nil
}
