package kubernetes

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekssdk "github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/test"
)

func TestReadClusterDetails_Caching(t *testing.T) {
	g := NewGomegaWithT(t)
	ctx := context.Background()

	// Setup mock EKS API response
	resp := &ekssdk.DescribeClusterOutput{
		Cluster: &types.Cluster{
			Name:     aws.String("test-cluster"),
			Endpoint: aws.String("https://test.endpoint"),
			CertificateAuthority: &types.Certificate{
				Data: aws.String(base64.StdEncoding.EncodeToString([]byte("test-ca"))),
			},
			KubernetesNetworkConfig: &types.KubernetesNetworkConfigResponse{
				ServiceIpv4Cidr: aws.String("10.0.0.0/16"),
			},
			Status: types.ClusterStatusActive,
		},
	}

	server := test.NewEKSDescribeClusterAPI(t, resp)
	config := aws.Config{
		BaseEndpoint: &server.URL,
		HTTPClient:   server.Client(),
	}

	provider := NewClusterProvider(config)

	node := &api.NodeConfig{
		Spec: api.NodeConfigSpec{
			Cluster: api.ClusterDetails{
				Name: "test-cluster",
			},
		},
	}

	// First call - should hit the API
	result1, err := provider.ReadClusterDetails(ctx, node)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result1.APIServerEndpoint).To(Equal("https://test.endpoint"))
	g.Expect(result1.CertificateAuthority).To(Equal([]byte("test-ca")))
	g.Expect(result1.CIDR).To(Equal("10.0.0.0/16"))

	// Shut down the server - any subsequent API calls should fail
	server.Close()

	// Modify the first result
	originalEndpoint := result1.APIServerEndpoint
	result1.APIServerEndpoint = "https://modified.endpoint"
	g.Expect(result1.APIServerEndpoint).To(Equal("https://modified.endpoint"), "First result should be modified")

	// Second call - should use cache since server is down
	result2, err := provider.ReadClusterDetails(ctx, node)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result2.APIServerEndpoint).To(Equal(originalEndpoint), "Second result should have original endpoint")
	g.Expect(result2.APIServerEndpoint).NotTo(Equal(result1.APIServerEndpoint), "Second result should not be affected by modifications to first result")

	// Modify the second result
	result2.APIServerEndpoint = "https://modified-again.endpoint"
	g.Expect(result2.APIServerEndpoint).To(Equal("https://modified-again.endpoint"), "Second result should be modified")

	// Third call - should still use cache since server is down
	result3, err := provider.ReadClusterDetails(ctx, node)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result3.APIServerEndpoint).To(Equal(originalEndpoint), "Third result should have original endpoint")
	g.Expect(result3.APIServerEndpoint).NotTo(Equal(result2.APIServerEndpoint), "Third result should not be affected by modifications to second result")
}
