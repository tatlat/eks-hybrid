package kubelet

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func (k *kubelet) ensureClusterDetails() error {
	if k.nodeConfig.Spec.Cluster.APIServerEndpoint == "" || k.nodeConfig.Spec.Cluster.CertificateAuthority == nil || k.nodeConfig.Spec.Cluster.CIDR == "" {
		eksClient := eks.NewFromConfig(*k.awsConfig)
		cluster, err := eksClient.DescribeCluster(context.Background(), &eks.DescribeClusterInput{
			Name: aws.String(k.nodeConfig.Spec.Cluster.Name),
		})
		if err != nil {
			return err
		}
		if cluster.Cluster.Status != types.ClusterStatusActive {
			return fmt.Errorf("eks cluster is not active")
		}
		if k.nodeConfig.Spec.Cluster.APIServerEndpoint == "" {
			k.nodeConfig.Spec.Cluster.APIServerEndpoint = *cluster.Cluster.Endpoint
		}

		if k.nodeConfig.Spec.Cluster.CertificateAuthority == nil {
			// CertificateAuthority from describeCluster api call returns base64 encoded data as a string
			// Decoding the string to byte array ensures the proper data format when writing to file
			decoded, err := base64.StdEncoding.DecodeString(*cluster.Cluster.CertificateAuthority.Data)
			if err != nil {
				return err
			}
			k.nodeConfig.Spec.Cluster.CertificateAuthority = decoded
		}

		if k.nodeConfig.Spec.Cluster.CIDR == "" {
			k.nodeConfig.Spec.Cluster.CIDR = *cluster.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr
		}
	}
	return nil
}
