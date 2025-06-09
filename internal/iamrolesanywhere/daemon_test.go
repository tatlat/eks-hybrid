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

	testCases := []struct {
		name         string
		envVars      map[string]string
		expectedFile string
	}{
		{
			name: "proxy enabled via HTTP_PROXY",
			envVars: map[string]string{
				"HTTP_PROXY": "http://proxy.example.com:8080",
			},
			expectedFile: "./testdata/expected-systemd-service-unit-with-proxy",
		},
		{
			name: "proxy enabled via HTTPS_PROXY",
			envVars: map[string]string{
				"HTTPS_PROXY": "https://proxy.example.com:8080",
			},
			expectedFile: "./testdata/expected-systemd-service-unit-with-proxy",
		},
		{
			name:         "proxy disabled",
			envVars:      map[string]string{},
			expectedFile: "./testdata/expected-systemd-service-unit",
		},
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tc.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				// Clean up environment variables
				for env := range tc.envVars {
					os.Unsetenv(env)
				}
			}()

			expect, err := os.ReadFile(tc.expectedFile)
			g.Expect(err).To(BeNil())

			service, err := iamrolesanywhere.GenerateUpdateSystemdService(node)
			g.Expect(err).To(BeNil())
			g.Expect(string(service)).To(BeComparableTo(string(expect)))
		})
	}
}
