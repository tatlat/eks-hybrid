// nolint
package e2e

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
)

type TestConfig struct {
	ClusterName   string `yaml:"clusterName"`
	ClusterRegion string `yaml:"clusterRegion"`
	HybridVpcID   string `yaml:"hybridVpcID"`
	NodeadmUrl    string `yaml:"nodeadmUrl"`
}

// newE2EAWSSession constructs AWS session for E2E tests.
func newE2EAWSSession(region string) (*session.Session, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}
	return sess, nil
}

// getKubernetesVersion returns kubernetes version of the cluster.
func getKubernetesVersion(ctx context.Context, eksClient *eks.EKS, clusterName string) (string, error) {
	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}

	result, err := eksClient.DescribeClusterWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe cluster: %v", err)
	}

	return *result.Cluster.Version, nil
}
