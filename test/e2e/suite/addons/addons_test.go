//go:build e2e
// +build e2e

package addons

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/aws/eks-hybrid/test/e2e"
	"github.com/aws/eks-hybrid/test/e2e/suite"
)

var (
	filePath    string
	suiteConfig *suite.SuiteConfiguration
)

const numberOfNodes = 1

func init() {
	flag.StringVar(&filePath, "filepath", "", "Path to configuration")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Addon Smoke Test Suite")
}

var _ = SynchronizedBeforeSuite(
	func(ctx context.Context) []byte {
		suiteConfig := suite.BeforeSuiteCredentialSetup(ctx, filePath)
		test := suite.BeforeVPCTest(ctx, &suiteConfig)
		credentialProviders := suite.AddClientsToCredentialProviders(suite.CredentialProviders(), test)
		osList := suite.OSProviderList(credentialProviders)

		// pick 3 random OS/Version/Provider combinations for metricsServer tests worker nodes
		nodesToCreate := make([]suite.NodeCreate, 0, numberOfNodes)

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
				InstanceSize: e2e.Large,
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
		var addonEc2Test *suite.AddonEc2Test

		BeforeEach(func(ctx context.Context) {
			addonEc2Test = &suite.AddonEc2Test{PeeredVPCTest: suite.BeforeVPCTest(ctx, suiteConfig)}
		})

		When("using ec2 instance as hybrid nodes", func() {
			It("runs node monitoring agent tests", func(ctx context.Context) {
				nodeMonitoringAgent := addonEc2Test.NewNodeMonitoringAgentTest()

				DeferCleanup(func(ctx context.Context) {
					Expect(nodeMonitoringAgent.Delete(ctx)).To(Succeed(), "should cleanup node monitoring agent successfully")
				})

				Expect(nodeMonitoringAgent.Create(ctx)).To(
					Succeed(), "node monitoring agent should have created successfully",
				)

				DeferCleanup(func(ctx context.Context) {
					// only print logs after addon successfully created
					report := CurrentSpecReport()
					if report.State.Is(types.SpecStateFailed) {
						err := nodeMonitoringAgent.PrintLogs(ctx)
						if err != nil {
							// continue cleanup even if logs collection fails
							GinkgoWriter.Printf("Failed to get node monitoring agent logs: %v\n", err)
						}
					}
				})

				Expect(nodeMonitoringAgent.Validate(ctx)).To(
					Succeed(), "node monitoring agent should have been validated successfully",
				)
			}, Label("node-monitoring-agent"))

			It("runs metrics server tests", func(ctx context.Context) {
				metricsServer := addonEc2Test.NewMetricsServerTest()

				DeferCleanup(func(ctx context.Context) {
					Expect(metricsServer.Delete(ctx)).To(Succeed(), "should cleanup metrics server successfully")
				})

				Expect(metricsServer.Create(ctx)).To(
					Succeed(), "metrics server should have created successfully",
				)

				DeferCleanup(func(ctx context.Context) {
					// only print logs after addon successfully created
					report := CurrentSpecReport()
					if report.State.Is(types.SpecStateFailed) {
						err := metricsServer.PrintLogs(ctx)
						if err != nil {
							// continue cleanup even if logs collection fails
							GinkgoWriter.Printf("Failed to get metrics server logs: %v\n", err)
						}
					}
				})

				Expect(metricsServer.Validate(ctx)).To(
					Succeed(), "metrics server should have been validated successfully",
				)
			}, Label("metrics-server"))

			It("runs kube state metrics tests", func(ctx context.Context) {
				kubeStateMetrics := addonEc2Test.NewKubeStateMetricsTest()

				DeferCleanup(func(ctx context.Context) {
					Expect(kubeStateMetrics.Delete(ctx)).To(Succeed(), "should cleanup kube state metrics successfully")
				})

				Expect(kubeStateMetrics.Create(ctx)).To(
					Succeed(), "kube state metrics should have created successfully",
				)

				DeferCleanup(func(ctx context.Context) {
					// only print logs after addon successfully created
					report := CurrentSpecReport()
					if report.State.Is(types.SpecStateFailed) {
						err := kubeStateMetrics.PrintLogs(ctx)
						if err != nil {
							// continue cleanup even if logs collection fails
							GinkgoWriter.Printf("Failed to get kube state metrics logs: %v\n", err)
						}
					}
				})

				Expect(kubeStateMetrics.Validate(ctx)).To(
					Succeed(), "kube state metrics should have been validated successfully",
				)
			}, Label("kube-state-metrics"))

			It("runs prometheus node exporter tests", func(ctx context.Context) {
				prometheusNodeExporter := addonEc2Test.NewPrometheusNodeExporterTest()

				DeferCleanup(func(ctx context.Context) {
					Expect(prometheusNodeExporter.Delete(ctx)).To(Succeed(), "should cleanup prometheus node exporter successfully")
				})

				Expect(prometheusNodeExporter.Create(ctx)).To(
					Succeed(), "prometheus node exporter should have created successfully",
				)

				DeferCleanup(func(ctx context.Context) {
					// only print logs after addon successfully created
					report := CurrentSpecReport()
					if report.State.Is(types.SpecStateFailed) {
						err := prometheusNodeExporter.PrintLogs(ctx)
						if err != nil {
							// continue cleanup even if logs collection fails
							GinkgoWriter.Printf("Failed to get prometheus node exporter logs: %v\n", err)
						}
					}
				})

				Expect(prometheusNodeExporter.Validate(ctx)).To(
					Succeed(), "prometheus node exporter should have been validated successfully",
				)
			}, Label("prometheus-node-exporter"))

			It("runs cert manager tests", func(ctx context.Context) {
				certManager := addonEc2Test.NewCertManagerTest()

				DeferCleanup(func(ctx context.Context) {
					Expect(certManager.Delete(ctx)).To(Succeed(), "should cleanup cert manager successfully")
				})

				Expect(certManager.Create(ctx)).To(
					Succeed(), "cert manager should have created successfully",
				)

				DeferCleanup(func(ctx context.Context) {
					// only print logs after addon successfully created
					report := CurrentSpecReport()
					if report.State.Is(types.SpecStateFailed) {
						err := certManager.PrintLogs(ctx)
						if err != nil {
							// continue cleanup even if logs collection fails
							GinkgoWriter.Printf("Failed to get cert manager logs: %v\n", err)
						}
					}
				})

				Expect(certManager.Validate(ctx)).To(
					Succeed(), "cert manager should have been validated successfully",
				)
			}, Label("cert-manager", "aws-pca-issuer"))


		})
	})
})
