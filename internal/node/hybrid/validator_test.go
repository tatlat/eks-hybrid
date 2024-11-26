package hybrid_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/node/hybrid"
)

func Test_HybridNodeProviderValidateConfig(t *testing.T) {
	testCases := []struct {
		name      string
		node      *api.NodeConfig
		wantError string
	}{
		{
			name: "happy path",
			node: &api.NodeConfig{
				Spec: api.NodeConfigSpec{
					Cluster: api.ClusterDetails{
						Region: "us-west-2",
						Name:   "my-cluster",
					},
					Hybrid: &api.HybridOptions{
						IAMRolesAnywhere: &api.IAMRolesAnywhere{
							NodeName:       "my-node",
							TrustAnchorARN: "trust-anchor-arn",
							ProfileARN:     "profile-arn",
							RoleARN:        "role-arn",
						},
					},
				},
			},
		},
		{
			name: "no node name",
			node: &api.NodeConfig{
				Spec: api.NodeConfigSpec{
					Cluster: api.ClusterDetails{
						Region: "us-west-2",
						Name:   "my-cluster",
					},
					Hybrid: &api.HybridOptions{
						IAMRolesAnywhere: &api.IAMRolesAnywhere{
							TrustAnchorARN: "trust-anchor-arn",
							ProfileARN:     "profile-arn",
							RoleARN:        "role-arn",
						},
					},
				},
			},
			wantError: "NodeName can't be empty in hybrid iam roles anywhere configuration",
		},
		{
			name: "node name too long",
			node: &api.NodeConfig{
				Spec: api.NodeConfigSpec{
					Cluster: api.ClusterDetails{
						Region: "us-west-2",
						Name:   "my-cluster",
					},
					Hybrid: &api.HybridOptions{
						IAMRolesAnywhere: &api.IAMRolesAnywhere{
							NodeName:       "my-node-too-long-1111111111111111111111111111111111111111111111111111",
							TrustAnchorARN: "trust-anchor-arn",
							ProfileARN:     "profile-arn",
							RoleARN:        "role-arn",
						},
					},
				},
			},
			wantError: "NodeName can't be longer than 64 characters in hybrid iam roles anywhere configuration",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			p, err := hybrid.NewHybridNodeProvider(tc.node, zap.NewNop())
			g.Expect(err).NotTo(HaveOccurred())

			err = p.ValidateConfig()
			if tc.wantError == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(MatchError(tc.wantError))
			}
		})
	}
}
