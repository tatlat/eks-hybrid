package iamrolesanywhere_test

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
)

func TestGenerateUpdateSystemdService(t *testing.T) {
	g := NewWithT(t)

	expect, err := os.ReadFile("./testdata/expected-systemd-service-unit")
	g.Expect(err).To(BeNil())
	if err != nil {
		t.Fatal(err)
	}

	node := &api.NodeConfig{
		Spec: api.NodeConfigSpec{
			Cluster: api.ClusterDetails{
				Region: "us-west-2",
			},
			Hybrid: &api.HybridOptions{
				IAMRolesAnywhere: &api.IAMRolesAnywhere{
					RoleARN:         "arn:aws:iam::123456789010:role/mockHybridNodeRole",
					ProfileARN:      "arn:aws:iam::123456789010:instance-profile/mockHybridNodeRole",
					TrustAnchorARN:  "arn:aws:acm-pca:us-west-2:123456789010:certificate-authority/fc32b514-4aca-4a4b-91a5-602294a6f4b7",
					NodeName:        "mock-hybrid-node",
					CertificatePath: "/etc/certificates/iam/pki/my-server.crt",
					PrivateKeyPath:  "/etc/certificates/iam/pki/my-server.key",
				},
			},
		},
	}

	service, err := iamrolesanywhere.GenerateUpdateSystemdService(node)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(service)).To(BeComparableTo(string(expect)))
}
