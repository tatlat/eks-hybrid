//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/eks-hybrid/internal/creds"
	"github.com/aws/eks-hybrid/internal/system"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var (
	filePath    string
	eksClient   *eks.EKS
	ec2Client   *ec2.EC2
	config      *TestConfig = &TestConfig{}
	testEntries []TableEntry
	kubeVersion string
)

func init() {
	flag.StringVar(&filePath, "filepath", "", "Path to configuration")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	g := NewGomegaWithT(t)

	g.Expect(filePath).NotTo(BeEmpty(), "-filePath flag is required") // Fail the test if the filepath flag is not provided
	g.Expect(loadTestConfig(config)).NotTo(HaveOccurred())
	g.Expect(generateTestEntries()).NotTo(BeEmpty())

	RunSpecs(t, "E2E Suite")
}

// loadTestConfig reads the configuration from the specified file path and unmarshals it into the TestConfig struct.
func loadTestConfig(config *TestConfig) error {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open configuration file: %v", err)
	}

	if err = yaml.Unmarshal(file, config); err != nil {
		return fmt.Errorf("failed to unmarshal configuration from YAML: %v", err)
	}

	return nil
}

var (
	osList              = []string{system.UbuntuOsName, system.AmazonOsName, system.RhelOsName}
	credentialProviders = []creds.CredentialProvider{creds.SsmCredentialProvider, creds.IamRolesAnywhereCredentialProvider}
)

func generateTestEntries() []TableEntry {
	for _, os := range osList {
		for _, provider := range credentialProviders {
			testEntries = append(testEntries, Entry(
				fmt.Sprintf("With OS %s and with Credential Provider %s", os, string(provider)),
				os,
				provider,
				Label(os, string(provider)),
			))
		}
	}
	return testEntries
}

var _ = BeforeSuite(func() {
	ctx := context.Background()
	awsSession, err := newE2EAWSSession(config.ClusterRegion)
	Expect(err).NotTo(HaveOccurred())

	eksClient = eks.New(awsSession)
	ec2Client = ec2.New(awsSession)

	// Get Kubernetes version for the given cluster name
	kubeVersion, err = getKubernetesVersion(ctx, eksClient, config.ClusterName)
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Hybrid Nodes", func() {
	When("using peered VPC", func() {
		DescribeTable("Joining a node",
			func(os string, provider creds.CredentialProvider) {
				Expect(os).NotTo(BeEmpty())
				Expect(provider).NotTo(BeEmpty())
				Expect(kubeVersion).NotTo(BeEmpty())

				fmt.Printf("Running test for OS: %s, Credential Provider: %s, Kubernetes Version: %s\n", os, string(provider), kubeVersion)
			}, testEntries)
	})
})
