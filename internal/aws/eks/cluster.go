package eks

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/validation"
)

// ReadCluster returns raw EKS cluster data from the AWS API with caching
func ReadCluster(ctx context.Context, config aws.Config, node *api.NodeConfig) (*types.Cluster, error) {
	client := eks.NewFromConfig(config)
	input := &eks.DescribeClusterInput{
		Name: &node.Spec.Cluster.Name,
	}
	clusterOutput, err := client.DescribeCluster(ctx, input)
	if err != nil {
		return nil, validation.WithRemediation(err,
			"Ensure the node has access and permissions to call DescribeCluster EKS API. "+
				"Check AWS credentials and IAM permissions.")
	}

	return clusterOutput.Cluster, nil
}

// ReadClusterDetails returns ClusterDetails with the API server endpoint, certificate authority, and CIDR block.
// If any of these are not set in the input node config, it retrieves them from the EKS API.
func ReadClusterDetails(ctx context.Context, config aws.Config, node *api.NodeConfig) (*api.ClusterDetails, error) {
	if node.Spec.Cluster.APIServerEndpoint != "" && node.Spec.Cluster.CertificateAuthority != nil && node.Spec.Cluster.CIDR != "" {
		return node.Spec.Cluster.DeepCopy(), nil
	}

	// Use the internal ReadCluster function to get raw cluster data
	cluster, err := ReadCluster(ctx, config, node)
	if err != nil {
		return nil, err
	}

	if cluster.Status != types.ClusterStatusActive {
		return nil, fmt.Errorf("eks cluster %s is not active", *cluster.Name)
	}

	clusterDetails := node.Spec.Cluster.DeepCopy()
	if clusterDetails.APIServerEndpoint == "" {
		clusterDetails.APIServerEndpoint = *cluster.Endpoint
	}

	if clusterDetails.CertificateAuthority == nil {
		// CertificateAuthority from describeCluster api call returns base64 encoded data as a string
		// Decoding the string to byte array ensures the proper data format when writing to file
		decoded, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
		if err != nil {
			return nil, err
		}
		clusterDetails.CertificateAuthority = decoded
	}

	if clusterDetails.CIDR == "" {
		clusterDetails.CIDR = *cluster.KubernetesNetworkConfig.ServiceIpv4Cidr
	}

	return clusterDetails, nil
}
