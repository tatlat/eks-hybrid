//go:build e2e
// +build e2e

package mixed_mode

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	ec2v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2v2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	smithyTime "github.com/aws/smithy-go/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/eks-hybrid/test/e2e"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/aws/eks-hybrid/test/e2e/suite"
)

var (
	filePath    string
	suiteConfig *suite.SuiteConfiguration

	// Namespace constants
	testNamespace = "default"

	// Labels for mixed mode test resources
	mixedModeLabels = map[string]string{
		"test-suite": "mixed-mode",
	}

	// Network CIDR constants (match defaults from create.go)
	cloudVPCCIDR  = "10.20.0.0/16"
	hybridVPCCIDR = "10.80.0.0/16"
	podCIDR       = "10.87.0.0/16"

	// Port constants
	httpPort int32 = 80
	dnsPort  int32 = 53

	// Timing constants
	crossVPCPropagationWait = 120 * time.Second

	// Node selector constants
	cloudNodeSelector  = map[string]string{"node.kubernetes.io/instance-type": "m5.large"}
	hybridNodeSelector = map[string]string{"eks.amazonaws.com/compute-type": "hybrid"}

	// Node label constants (derived from selectors)
	cloudNodeLabelKey, cloudNodeLabelValue   = getFirstKeyValue(cloudNodeSelector)
	hybridNodeLabelKey, hybridNodeLabelValue = getFirstKeyValue(hybridNodeSelector)
)

// getFirstKeyValue extracts the first key-value pair from a map
func getFirstKeyValue(selector map[string]string) (string, string) {
	for k, v := range selector {
		return k, v
	}
	return "", ""
}

func init() {
	flag.StringVar(&filePath, "filepath", "", "Path to configuration")
}

func TestMixedModeE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mixed Mode E2E Suite")
}

var _ = SynchronizedBeforeSuite(
	// This function only runs once, on the first process
	func(ctx context.Context) []byte {
		suiteConfig := suite.BeforeSuiteCredentialSetup(ctx, filePath)

		test := suite.BeforeVPCTest(ctx, &suiteConfig)
		credentialProviders := suite.AddClientsToCredentialProviders(suite.CredentialProviders(), test)
		osProviderList := suite.OSProviderList(credentialProviders)
		randomOSProvider := osProviderList[rand.Intn(len(osProviderList))]

		hybridNode := suite.NodeCreate{
			InstanceName: "mixed-mode-hybrid",
			InstanceSize: e2e.Large,
			NodeName:     "mixed-mode-hybrid",
			OS:           randomOSProvider.OS,
			Provider:     randomOSProvider.Provider,
			ComputeType:  e2e.CPUInstance,
		}
		suite.CreateNodes(ctx, test, []suite.NodeCreate{hybridNode})

		Expect(test.CreateManagedNodeGroups(ctx)).To(Succeed(), "managed node group should be created successfully")

		// Ensure mixed mode connectivity by adding required security group rules
		err := ensureMixedModeConnectivity(ctx, test)
		Expect(err).NotTo(HaveOccurred(), "Mixed mode connectivity rules should be added successfully")

		suiteJson, err := yaml.Marshal(suiteConfig)
		Expect(err).NotTo(HaveOccurred(), "suite config should be marshalled successfully")
		return suiteJson
	},
	// This function runs on all processes
	func(ctx context.Context, data []byte) {
		// add a small sleep to add jitter to the start of each test
		randomSleep := rand.Intn(10)
		err := smithyTime.SleepWithContext(ctx, time.Duration(randomSleep)*time.Second)
		Expect(err).NotTo(HaveOccurred(), "failed to sleep")
		suiteConfig = suite.BeforeSuiteCredentialUnmarshal(ctx, data)
	},
)

var _ = Describe("Mixed Mode Testing", func() {
	When("hybrid and cloud-managed nodes coexist", func() {
		var test *suite.PeeredVPCTest

		BeforeEach(func(ctx context.Context) {
			test = suite.BeforeVPCTest(ctx, suiteConfig)

			// Comprehensive cleanup before each test
			test.Logger.Info("Running comprehensive cleanup to ensure clean state")
			cleanupTestResources(ctx, test, mixedModeLabels)
		})

		AfterEach(func(ctx context.Context) {
			test.Logger.Info("Running comprehensive cleanup after test")
			cleanupTestResources(ctx, test, mixedModeLabels)
		})

		Context("Pod-to-Pod Communication", func() {
			It("should enable cross-VPC pod-to-pod communication from hybrid to cloud nodes", func(ctx context.Context) {
				// Find cloud node and create nginx pod
				cloudNodeName, _ := kubernetes.FindNodeWithLabel(ctx, test.K8sClient.Interface, cloudNodeLabelKey, cloudNodeLabelValue, test.Logger)

				err := kubernetes.CreateNginxPodInNode(ctx, test.K8sClient.Interface, cloudNodeName, testNamespace, test.Cluster.Region, test.Logger, "nginx-cloud", mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating cloud pod")
				test.Logger.Info("Cloud pod created and ready", "name", "nginx-cloud")

				// Find hybrid node and create client nginx pod
				hybridNodeName, _ := kubernetes.FindNodeWithLabel(ctx, test.K8sClient.Interface, hybridNodeLabelKey, hybridNodeLabelValue, test.Logger)

				err = kubernetes.CreateNginxPodInNode(ctx, test.K8sClient.Interface, hybridNodeName, testNamespace, test.Cluster.Region, test.Logger, "test-client-hybrid", mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating client pod")
				test.Logger.Info("Client pod created and ready", "name", "test-client-hybrid")

				test.Logger.Info("Testing cross-VPC pod-to-pod connectivity (hybrid → cloud)")
				err = kubernetes.TestPodToPodConnectivity(ctx, test.K8sClientConfig, test.K8sClient.Interface, "test-client-hybrid", "nginx-cloud", testNamespace, test.Logger)
				Expect(err).NotTo(HaveOccurred(), "testing pod-to-pod connectivity")

				test.Logger.Info("Cross-VPC pod-to-pod communication test completed successfully")
			})

			It("should enable cross-VPC pod-to-pod communication from cloud to hybrid nodes", func(ctx context.Context) {
				// Find hybrid node and create nginx pod
				hybridNodeName, _ := kubernetes.FindNodeWithLabel(ctx, test.K8sClient.Interface, hybridNodeLabelKey, hybridNodeLabelValue, test.Logger)

				err := kubernetes.CreateNginxPodInNode(ctx, test.K8sClient.Interface, hybridNodeName, testNamespace, test.Cluster.Region, test.Logger, "nginx-hybrid-reverse", mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating hybrid pod")
				test.Logger.Info("Hybrid pod created and ready", "name", "nginx-hybrid-reverse")

				// Find cloud node and create client nginx pod
				cloudNodeName, _ := kubernetes.FindNodeWithLabel(ctx, test.K8sClient.Interface, cloudNodeLabelKey, cloudNodeLabelValue, test.Logger)

				err = kubernetes.CreateNginxPodInNode(ctx, test.K8sClient.Interface, cloudNodeName, testNamespace, test.Cluster.Region, test.Logger, "test-client-cloud", mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating client pod")
				test.Logger.Info("Cloud client pod created and ready", "name", "test-client-cloud")

				err = kubernetes.TestPodToPodConnectivity(ctx, test.K8sClientConfig, test.K8sClient.Interface, "test-client-cloud", "nginx-hybrid-reverse", testNamespace, test.Logger)
				Expect(err).NotTo(HaveOccurred(), "testing pod-to-pod connectivity")

				test.Logger.Info("Cross-VPC pod-to-pod communication test completed successfully")
			})
		})

		Context("Cross-VPC Service Discovery", func() {
			It("should enable cross-VPC service discovery from hybrid to cloud services", func(ctx context.Context) {
				// Create deployment
				_, err := kubernetes.CreateDeployment(ctx, test.K8sClient.Interface, "nginx-service-cloud", testNamespace, test.Cluster.Region, cloudNodeSelector, httpPort, 1, test.Logger, mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating deployment")

				// Create service (port 80 to target port 80)
				service, err := kubernetes.CreateService(ctx, test.K8sClient.Interface, "nginx-service-cloud", testNamespace, map[string]string{"app": "nginx-service-cloud"}, httpPort, httpPort, test.Logger, mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating service")

				// Find hybrid node and create client nginx pod
				hybridNodeName, _ := kubernetes.FindNodeWithLabel(ctx, test.K8sClient.Interface, hybridNodeLabelKey, hybridNodeLabelValue, test.Logger)

				err = kubernetes.CreateNginxPodInNode(ctx, test.K8sClient.Interface, hybridNodeName, testNamespace, test.Cluster.Region, test.Logger, "test-client-hybrid-service", mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating client pod")
				test.Logger.Info("Client pod created and ready", "name", "test-client-hybrid-service")

				err = kubernetes.WaitForServiceReady(ctx, test.K8sClient.Interface, service.Name, testNamespace, test.Logger)
				Expect(err).NotTo(HaveOccurred(), "waiting for service to be ready")

				// Allow time for network rules and DNS to fully propagate across VPCs
				test.Logger.Info("Waiting for network rules and DNS to propagate across VPCs")
				time.Sleep(crossVPCPropagationWait)

				// Test service connectivity with integrated DNS resolution testing (port 80)
				err = kubernetes.TestServiceConnectivityWithRetries(ctx, test.K8sClientConfig, test.K8sClient.Interface, "test-client-hybrid-service", service.Name, testNamespace, httpPort, test.Logger)
				Expect(err).NotTo(HaveOccurred(), "testing service connectivity")

				test.Logger.Info("Cross-VPC service discovery test (hybrid → cloud) completed successfully")
			})

			It("should enable cross-VPC service discovery from cloud to hybrid services", func(ctx context.Context) {
				test.Logger.Info("Testing bidirectional service discovery (cloud → hybrid service)")

				_, err := kubernetes.CreateDeployment(ctx, test.K8sClient.Interface, "nginx-service-hybrid", testNamespace, test.Cluster.Region, hybridNodeSelector, httpPort, 1, test.Logger, mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating deployment")

				service, err := kubernetes.CreateService(ctx, test.K8sClient.Interface, "nginx-service-hybrid", testNamespace, map[string]string{"app": "nginx-service-hybrid"}, httpPort, httpPort, test.Logger, mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating service")

				// Find cloud node and create client nginx pod
				cloudNodeName, _ := kubernetes.FindNodeWithLabel(ctx, test.K8sClient.Interface, cloudNodeLabelKey, cloudNodeLabelValue, test.Logger)

				err = kubernetes.CreateNginxPodInNode(ctx, test.K8sClient.Interface, cloudNodeName, testNamespace, test.Cluster.Region, test.Logger, "test-client-cloud-bidirectional", mixedModeLabels)
				Expect(err).NotTo(HaveOccurred(), "creating client pod")
				test.Logger.Info("Client pod created and ready", "name", "test-client-cloud-bidirectional")

				err = kubernetes.WaitForServiceReady(ctx, test.K8sClient.Interface, service.Name, testNamespace, test.Logger)
				Expect(err).NotTo(HaveOccurred(), "waiting for service to be ready")
				// Allow time for network rules and DNS to fully propagate across VPCs
				test.Logger.Info("Waiting for network rules and DNS to propagate across VPCs")
				time.Sleep(crossVPCPropagationWait)
				// Test service connectivity with integrated DNS resolution testing (port 80)
				err = kubernetes.TestServiceConnectivityWithRetries(ctx, test.K8sClientConfig, test.K8sClient.Interface, "test-client-cloud-bidirectional", service.Name, testNamespace, httpPort, test.Logger)
				Expect(err).NotTo(HaveOccurred(), "testing service connectivity")

				test.Logger.Info("Bidirectional service discovery test (cloud → hybrid) completed successfully")
			})
		})
	})
})

// cleanupTestResources performs comprehensive cleanup using provided labels
func cleanupTestResources(ctx context.Context, test *suite.PeeredVPCTest, labels map[string]string) {
	// Construct label selector from provided labels map
	var labelParts []string
	for key, value := range labels {
		labelParts = append(labelParts, fmt.Sprintf("%s=%s", key, value))
	}
	labelSelector := strings.Join(labelParts, ",")

	test.Logger.Info("Starting comprehensive cleanup using label", "selector", labelSelector)

	// Clean up pods
	err := kubernetes.DeletePodsWithLabels(ctx, test.K8sClient.Interface, testNamespace, labelSelector, test.Logger)
	if err != nil {
		test.Logger.Info("Failed to delete pods with selector", "selector", labelSelector, "error", err.Error())
	}

	// Clean up services
	err = kubernetes.DeleteServicesWithLabels(ctx, test.K8sClient.Interface, testNamespace, labelSelector, test.Logger)
	if err != nil {
		test.Logger.Info("Failed to delete services with selector", "selector", labelSelector, "error", err.Error())
	}

	// Clean up deployments
	err = kubernetes.DeleteDeploymentsWithLabels(ctx, test.K8sClient.Interface, testNamespace, labelSelector, test.Logger)
	if err != nil {
		test.Logger.Info("Failed to delete deployments with selector", "selector", labelSelector, "error", err.Error())
	}

	test.Logger.Info("Comprehensive cleanup completed", "selector", labelSelector)
}

// ensureMixedModeConnectivity adds required security group rules for mixed mode testing
func ensureMixedModeConnectivity(ctx context.Context, test *suite.PeeredVPCTest) error {
	clusterSG, hybridSG, err := getSecurityGroupsForMixedMode(ctx, test)
	if err != nil {
		return fmt.Errorf("failed to get security groups: %w", err)
	}

	rules := []struct {
		protocol string
		port     int32
		cidr     string
		desc     string
	}{
		// HTTP connectivity for pod-to-pod and service tests
		{"tcp", httpPort, cloudVPCCIDR, "HTTP 80 from cloud VPC"},
		{"tcp", httpPort, podCIDR, "HTTP 80 from pod CIDR"},
		{"tcp", httpPort, hybridVPCCIDR, "HTTP 80 from hybrid subnet"},
		// DNS connectivity for cross-VPC CoreDNS resolution
		{"tcp", dnsPort, cloudVPCCIDR, "DNS TCP from cloud VPC"},
		{"udp", dnsPort, cloudVPCCIDR, "DNS UDP from cloud VPC"},
		{"tcp", dnsPort, podCIDR, "DNS TCP from pod CIDR"},
		{"udp", dnsPort, podCIDR, "DNS UDP from pod CIDR"},
		{"tcp", dnsPort, hybridVPCCIDR, "DNS TCP from hybrid subnet"},
		{"udp", dnsPort, hybridVPCCIDR, "DNS UDP from hybrid subnet"},
	}

	// Apply rules to both cluster security group and hybrid node security group
	securityGroups := []*string{clusterSG, hybridSG}

	for _, sg := range securityGroups {
		test.Logger.Info("Adding security group rules", "sgId", *sg)
		for _, rule := range rules {
			ipPermission := &ec2v2types.IpPermission{
				IpProtocol: &rule.protocol,
				FromPort:   &rule.port,
				ToPort:     &rule.port,
				IpRanges: []ec2v2types.IpRange{
					{CidrIp: &rule.cidr},
				},
			}
			_, err := test.EC2Client.AuthorizeSecurityGroupIngress(ctx, &ec2v2.AuthorizeSecurityGroupIngressInput{
				GroupId:       sg,
				IpPermissions: []ec2v2types.IpPermission{*ipPermission},
			})

			// Ignore "already exists" errors since rules might already be in place
			if err != nil && !strings.Contains(err.Error(), "InvalidPermission.Duplicate") {
				return fmt.Errorf("failed to add %s rule for %s on SG %s: %w", rule.protocol, rule.cidr, *sg, err)
			}
		}
	}

	if err := ensureCoreDNSDistribution(ctx, test); err != nil {
		test.Logger.Info("CoreDNS distribution configuration had issues - mixed mode will still work", "error", err.Error())
	} else {
		test.Logger.Info("CoreDNS distribution configured for optimal mixed mode performance")
	}

	return nil
}

// getSecurityGroupsForMixedMode returns both cluster and hybrid node security groups
func getSecurityGroupsForMixedMode(ctx context.Context, test *suite.PeeredVPCTest) (*string, *string, error) {
	clusterName := test.Cluster.Name
	cluster, err := test.EKSClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: &clusterName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe cluster: %w", err)
	}

	clusterSG := cluster.Cluster.ResourcesVpcConfig.ClusterSecurityGroupId
	if clusterSG == nil {
		return nil, nil, fmt.Errorf("cluster security group ID not found")
	}

	test.Logger.Info("Found EKS cluster security group", "sgId", *clusterSG)

	hybridSG, err := findHybridNodeSecurityGroup(ctx, test)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find hybrid node security group: %w", err)
	}

	test.Logger.Info("Found hybrid node security group", "sgId", hybridSG)
	return clusterSG, &hybridSG, nil
}

// findHybridNodeSecurityGroup finds the security group of the hybrid node
func findHybridNodeSecurityGroup(ctx context.Context, test *suite.PeeredVPCTest) (string, error) {
	// Find hybrid node instance by name tag (created with InstanceName: "mixed-mode-hybrid")
	instances, err := test.EC2Client.DescribeInstances(ctx, &ec2v2.DescribeInstancesInput{
		Filters: []ec2v2types.Filter{
			{
				Name:   &[]string{"tag:Name"}[0],
				Values: []string{"mixed-mode-hybrid"},
			},
			{
				Name:   &[]string{"instance-state-name"}[0],
				Values: []string{"running"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe instances: %w", err)
	}

	if len(instances.Reservations) == 0 || len(instances.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("no hybrid node instance found with name 'mixed-mode-hybrid'")
	}

	instance := instances.Reservations[0].Instances[0]
	if len(instance.SecurityGroups) == 0 {
		return "", fmt.Errorf("no security groups found for hybrid node instance %s", *instance.InstanceId)
	}

	return *instance.SecurityGroups[0].GroupId, nil
}

// ensureCoreDNSDistribution configures CoreDNS for optimal mixed mode distribution using AWS best practices
func ensureCoreDNSDistribution(ctx context.Context, test *suite.PeeredVPCTest) error {
	test.Logger.Info("Configuring CoreDNS distribution using AWS best practices")

	// Step 1: Label hybrid nodes with topology.kubernetes.io/zone=onprem
	err := labelHybridNodesForTopology(ctx, test)
	if err != nil {
		return fmt.Errorf("failed to label hybrid nodes: %w", err)
	}

	// Step 2: Apply AWS recommended preferred anti-affinity configuration
	coreDNSPatch := `{
		"spec": {
			"replicas": 2,
			"template": {
				"spec": {
					"affinity": {
						"podAntiAffinity": {
							"preferredDuringSchedulingIgnoredDuringExecution": [
								{
									"weight": 100,
									"podAffinityTerm": {
										"labelSelector": {
											"matchExpressions": [
												{
													"key": "k8s-app",
													"operator": "In",
													"values": ["kube-dns"]
												}
											]
										},
										"topologyKey": "kubernetes.io/hostname"
									}
								},
								{
									"weight": 50,
									"podAffinityTerm": {
										"labelSelector": {
											"matchExpressions": [
												{
													"key": "k8s-app",
													"operator": "In", 
													"values": ["kube-dns"]
												}
											]
										},
										"topologyKey": "topology.kubernetes.io/zone"
									}
								}
							]
						}
					}
				}
			}
		}
	}`

	_, err = test.K8sClient.Interface.AppsV1().Deployments("kube-system").Patch(ctx, "coredns",
		types.MergePatchType, []byte(coreDNSPatch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to configure CoreDNS preferred anti-affinity: %w", err)
	}

	// Step 3: Configure kube-dns service for traffic distribution
	serviceTrafficPatch := `{
		"spec": {
			"trafficDistribution": "PreferClose"
		}
	}`

	_, err = test.K8sClient.Interface.CoreV1().Services("kube-system").Patch(ctx, "kube-dns",
		types.MergePatchType, []byte(serviceTrafficPatch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to configure kube-dns traffic distribution: %w", err)
	}

	test.Logger.Info("CoreDNS configured with AWS best practices: preferred anti-affinity, traffic distribution, and optimal pod placement")
	return nil
}

// labelHybridNodesForTopology adds topology.kubernetes.io/zone=onprem label to hybrid nodes
func labelHybridNodesForTopology(ctx context.Context, test *suite.PeeredVPCTest) error {
	// Find all hybrid nodes
	nodes, err := kubernetes.ListNodesWithLabels(ctx, test.K8sClient.Interface, "eks.amazonaws.com/compute-type=hybrid")
	if err != nil {
		return fmt.Errorf("failed to list hybrid nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		test.Logger.Info("No hybrid nodes found to label")
		return nil
	}

	// Label each hybrid node with topology zone
	for _, node := range nodes.Items {
		labelPatch := `{
			"metadata": {
				"labels": {
					"topology.kubernetes.io/zone": "onprem"
				}
			}
		}`

		err = kubernetes.PatchNode(ctx, test.K8sClient.Interface, node.Name, []byte(labelPatch), test.Logger)
		if err != nil {
			return fmt.Errorf("failed to label hybrid node %s: %w", node.Name, err)
		}

		test.Logger.Info("Labeled hybrid node with topology zone", "node", node.Name, "zone", "onprem")
	}

	return nil
}
