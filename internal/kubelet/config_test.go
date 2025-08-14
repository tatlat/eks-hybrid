package kubelet

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/eks-hybrid/internal/api"
)

func TestHybridCloudProvider(t *testing.T) {
	nodeConfig := api.NodeConfig{
		Spec: api.NodeConfigSpec{
			Cluster: api.ClusterDetails{
				Name:   "my-cluster",
				Region: "us-west-2",
			},
			Hybrid: &api.HybridOptions{
				IAMRolesAnywhere: &api.IAMRolesAnywhere{
					NodeName:       "my-node",
					TrustAnchorARN: "arn:aws:iam::222211113333:role/AmazonEKSConnectorAgentRole",
					ProfileARN:     "dummy-profile-arn",
					RoleARN:        "dummy-assume-role-arn",
				},
			},
		},
		Status: api.NodeConfigStatus{
			Hybrid: api.HybridDetails{
				NodeName: "my-node",
			},
		},
	}
	expectedProviderId := "eks-hybrid:///us-west-2/my-cluster/my-node"
	kubeletArgs := make(map[string]string)
	kubeletConfig := defaultKubeletSubConfig()
	kubeletConfig.withHybridCloudProvider(&nodeConfig, kubeletArgs)
	assert.Equal(t, kubeletArgs["cloud-provider"], "")
	assert.Equal(t, kubeletArgs["hostname-override"], nodeConfig.Status.Hybrid.NodeName)
	assert.Equal(t, *kubeletConfig.ProviderID, expectedProviderId)
}

func TestHybridLabels(t *testing.T) {
	nodeConfig := api.NodeConfig{
		Spec: api.NodeConfigSpec{
			Cluster: api.ClusterDetails{
				Name:   "my-cluster",
				Region: "us-west-2",
			},
			Hybrid: &api.HybridOptions{
				IAMRolesAnywhere: &api.IAMRolesAnywhere{
					NodeName:       "my-node",
					TrustAnchorARN: "arn:aws:iam::222211113333:role/AmazonEKSConnectorAgentRole",
					ProfileARN:     "dummy-profile-arn",
					RoleARN:        "dummy-assume-role-arn",
				},
			},
		},
	}
	expectedLabels := "eks.amazonaws.com/compute-type=hybrid,eks.amazonaws.com/hybrid-credential-provider=iam-ra"
	kubeletArgs := make(map[string]string)
	kubeletConfig := defaultKubeletSubConfig()
	kubeletConfig.withHybridNodeLabels(&nodeConfig, kubeletArgs)
	assert.Equal(t, kubeletArgs["node-labels"], expectedLabels)
}

func TestResolvConf(t *testing.T) {
	resolvConfPath := "/dummy/path/to/resolv.conf"
	kubeletConfig := defaultKubeletSubConfig()
	kubeletConfig.withResolvConf(resolvConfPath)
	assert.Equal(t, kubeletConfig.ResolvConf, resolvConfPath)
}
