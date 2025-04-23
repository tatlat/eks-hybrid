package smoketest

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/aws/eks-hybrid/test/e2e"
	"github.com/aws/eks-hybrid/test/e2e/addon"
	"github.com/aws/eks-hybrid/test/e2e/suite"
)

var (
	filePath     string
	suiteConfig  *suite.SuiteConfiguration
	addonsToTest []addon.AddonIface
)

const numberOfNodes = 1

func init() {
	flag.StringVar(&filePath, "filepath", "", "Path to configuration")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "addonTest Suite")
}

var _ = SynchronizedBeforeSuite(
	func(ctx context.Context) []byte {
		suiteConfig := suite.BeforeSuiteCredentialSetup(ctx, filePath)
		test := suite.BeforeVPCTest(ctx, &suiteConfig)
		credentialProviders := suite.AddClientsToCredentialProviders(suite.CredentialProviders(), test)
		osList := suite.OSProviderList(credentialProviders)

		// pick 3 random OS/Version/Provider combinations for addonTest tests worker nodes
		nodesToCreate := []suite.NodeCreate{}

		// Add more addons here
		addonsToTest = []addon.AddonIface{
			addon.NewMetricsServerAddon(suiteConfig.TestConfig.ClusterName, test.K8sClientConfig),
		}

		rand.Shuffle(len(osList), func(i, j int) {
			osList[i], osList[j] = osList[j], osList[i]
		})

		for i := range numberOfNodes {
			os := osList[i].OS
			provider := osList[i].Provider
			nodesToCreate = append(nodesToCreate, suite.NodeCreate{
				OS:           os,
				Provider:     provider,
				InstanceName: test.InstanceName("addon-smoke-test", os, provider),
				InstanceSize: e2e.XLarge,
				NodeName:     fmt.Sprintf("addon-test-node-%s-%s", provider.Name(), os.Name()),
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

		BeforeEach(func(ctx context.Context) {
			test = suite.BeforeVPCTest(ctx, suiteConfig)
		})

		When("using ec2 instance as hybrid nodes", func() {
			DescribeTable("runs addons",
				func(ctx context.Context, testAddon addon.AddonIface) {
					test.Logger.Info("Running addon test for " + testAddon.GetName())

					addonTest := addon.NewAddonTest(test.K8sClientConfig, test.K8sClient, test.EksClient, test.Logger, testAddon)
					DeferCleanup(func(ctx context.Context) {
						Expect(addonTest.CollectLogs(ctx)).To(Succeed(), "should collect addon logs successfully")

						Expect(addonTest.Cleanup(ctx)).To(Succeed(), "should cleanup addon successfully")
					})

					Expect(addonTest.Run(ctx)).To(
						Succeed(), "addon test should have run successfully",
					)
				},
				func() []TableEntry {
					entries := make([]TableEntry, len(addonsToTest))
					for i, addon := range addonsToTest {
						entries[i] = Entry(
							addon.GetName(),
							addon,
						)
					}
					return entries
				}(),
				Label("addonTest"),
			)
		})
	})
})
