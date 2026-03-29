//go:build e2e
// +build e2e

package conformance

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	clientgo "k8s.io/client-go/kubernetes"

	"github.com/aws/eks-hybrid/test/e2e"
	"github.com/aws/eks-hybrid/test/e2e/constants"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/aws/eks-hybrid/test/e2e/suite"
)

//go:embed testdata/repo-list.yaml
var repoListTemplate string

var (
	filePath    string
	suiteConfig *suite.SuiteConfiguration
)

const (
	numberOfNodes            = 3
	defaultConformanceReport = "junit_01.xml"
	conformanceReport        = "junit-conformance.xml"
)

func init() {
	flag.StringVar(&filePath, "filepath", "", "Path to configuration")
}

// generateRepoListFile generates a repo-list.yaml file from the template with actual values
func generateRepoListFile(region, ecrAccount, dnsSuffix, outputDir string) (string, error) {
	// Replace template placeholders with actual values
	replacer := strings.NewReplacer(
		"{{ECR_ACCOUNT_ID}}", ecrAccount,
		"{{REGION}}", region,
		"{{DNS_SUFFIX}}", dnsSuffix,
	)
	content := replacer.Replace(repoListTemplate)

	// Write the generated file to the output directory
	repoListPath := filepath.Join(outputDir, "repo-list.yaml")
	if err := os.WriteFile(repoListPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("failed to write repo-list.yaml: %w", err)
	}

	return repoListPath, nil
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conformance Suite")
}

var _ = SynchronizedBeforeSuite(
	// This function only runs once, on the first process
	// Here is where we want to run the setup infra code that should only run once
	// Whatever information we want to pass from this first process to all the processes
	// needs to be serialized into a byte slice
	// In this case, we use a struct marshalled in json.
	// We also create 3 nodes to be used by the conformance tests
	func(ctx context.Context) []byte {
		suiteConfig := suite.BeforeSuiteCredentialSetup(ctx, filePath)
		test := suite.BeforeVPCTest(ctx, &suiteConfig)
		credentialProviders := suite.AddClientsToCredentialProviders(suite.CredentialProviders(), test)
		osList := suite.OSProviderList(credentialProviders, test.Cluster.Region)

		// pick 3 random OS/Version/Provider combinations for conformance tests worker nodes
		nodesToCreate := []suite.NodeCreate{}

		rand.Shuffle(len(osList), func(i, j int) {
			osList[i], osList[j] = osList[j], osList[i]
		})

		for i := range numberOfNodes {
			os := osList[i].OS
			provider := osList[i].Provider
			nodesToCreate = append(nodesToCreate, suite.NodeCreate{
				OS:           os,
				Provider:     provider,
				InstanceName: test.InstanceName("conformance", os.Name(), string(provider.Name())),
				InstanceSize: e2e.XLarge,
				NodeName:     "conformance" + "-node-" + string(provider.Name()) + "-" + os.Name(),
			})
		}
		suite.CreateNodes(ctx, test, nodesToCreate)

		suiteJson, err := yaml.Marshal(suiteConfig)
		Expect(err).NotTo(HaveOccurred(), "suite config should be marshalled successfully")
		return suiteJson
	},
	// This function runs on all processes, and it receives the data from
	// the first process (a json serialized struct)
	// The only thing that we want to do here is unmarshal the data into
	// a struct that we can make accessible from the tests. We leave the rest
	// for the per tests setup code.
	func(ctx context.Context, data []byte) {
		suiteConfig = suite.BeforeSuiteCredentialUnmarshal(ctx, data)
	},
)

var _ = Describe("Hybrid Nodes", func() {
	When("using peered VPC", func() {
		var test *suite.PeeredVPCTest

		// Here is where we setup everything we need for the test. This includes
		// reading the setup output shared by the "before suite" code. This is the only place
		// that should be reading that global state, anything needed in the test code should
		// be passed down through "local" variable state. The global state should never be modified.
		BeforeEach(func(ctx context.Context) {
			test = suite.BeforeVPCTest(ctx, suiteConfig)
		})

		When("using ec2 instance as hybrid nodes", func() {
			It("runs conformance", Serial, SpecTimeout(3*time.Hour), func(ctx context.Context) {
				test.Logger.Info("Running NodeConformance tests...")
				k8sClient, err := clientgo.NewForConfig(test.K8sClientConfig)
				Expect(err).NotTo(HaveOccurred(), "should create kubernetes client successfully")

				outputFolder := filepath.Join(test.ArtifactsPath, "conformance")
				conformanceReportPath := filepath.Join(outputFolder, conformanceReport)
				Expect(os.MkdirAll(outputFolder, 0o755)).To(Succeed(), "should create output folder successfully")
				AddReportEntry(constants.TestConformancePath, conformanceReportPath)

				// Configure registry mappings for conformance tests
				// Generate repo-list.yaml dynamically based on test configuration
				repoListPath, err := generateRepoListFile(test.Cluster.Region, test.EcrAccount, test.DNSSuffix, outputFolder)
				Expect(err).NotTo(HaveOccurred(), "should generate repo-list.yaml successfully")

				conformance := kubernetes.NewConformanceTest(test.K8sClientConfig, k8sClient, test.Logger, test.Cluster.Region, test.EcrAccount, test.DNSSuffix,
					kubernetes.WithOutputDir(outputFolder),
					kubernetes.WithTestRepoList(repoListPath))
				DeferCleanup(func(ctx context.Context) {
					Expect(conformance.CollectLogs(ctx)).To(Succeed(), "should collect logs successfully")
					Expect(os.Rename(filepath.Join(outputFolder, defaultConformanceReport), conformanceReportPath)).To(Succeed(), "should rename conformance report successfully")

					Expect(conformance.Cleanup(ctx)).To(Succeed(), "should cleanup conformance successfully")
					Expect(kubernetes.WaitForNamespaceToBeDeleted(ctx, k8sClient, conformance.Namespace)).To(Succeed(), "conformance namespace should be deleted successfully")
				})

				Expect(conformance.Run(ctx)).To(
					Succeed(), "node conformance should have run successfully",
				)

				exitCode, err := conformance.FetchExitCode(ctx)
				Expect(err).NotTo(HaveOccurred(), "should fetch exit code successfully")
				Expect(exitCode).To(Equal(0), "conformance should have run successfully")
			}, Label("conformance"))
		})
	})
})
