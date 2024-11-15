//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/eks-hybrid/internal/creds"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

const (
	ec2InstanceType = "t2.large"
	ec2VolumeSize   = int64(30)
)

var (
	filePath string
	config   *TestConfig = &TestConfig{}
	logger   logr.Logger
)

type TestConfig struct {
	ClusterName   string `yaml:"clusterName"`
	ClusterRegion string `yaml:"clusterRegion"`
	HybridVpcID   string `yaml:"hybridVpcID"`
	NodeadmUrlAMD string `yaml:"nodeadmUrlAMD"`
	NodeadmUrlARM string `yaml:"nodeadmUrlARM"`
}

func init() {
	flag.StringVar(&filePath, "filepath", "", "Path to configuration")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	g := NewGomegaWithT(t)

	g.Expect(filePath).NotTo(BeEmpty(), "-filepath flag is required") // Fail the test if the filepath flag is not provided
	g.Expect(loadTestConfig(config)).NotTo(HaveOccurred())

	logger = NewLogger()

	RunSpecs(t, "E2E Suite")
}

// loadTestConfig reads the configuration from the specified file path and unmarshals it into the TestConfig struct.
func loadTestConfig(config *TestConfig) error {
	file, err := os.ReadFile(filePath)
	Expect(err).NotTo(HaveOccurred(), "expected to read configuration file")

	if err = yaml.Unmarshal(file, config); err != nil {
		return fmt.Errorf("failed to unmarshal configuration from YAML: %v", err)
	}

	return nil
}

func removeSpecialChars(input string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return re.ReplaceAllString(input, "")
}

func enabledCredentialsProviders(providers []NodeadmCredentialsProvider) []NodeadmCredentialsProvider {
	filter := GinkgoLabelFilter()
	providerList := []NodeadmCredentialsProvider{}

	for _, provider := range providers {
		if strings.Contains(filter, string(provider.Name())) {
			providerList = append(providerList, provider)
		}
	}
	return providerList
}

func getCredentialProviderNames(providers []NodeadmCredentialsProvider) string {
	var names []string
	for _, provider := range providers {
		names = append(names, string(provider.Name()))
	}
	return strings.Join(names, ", ")
}

type peeredVPCTest struct {
	awsSession *session.Session
	eksClient  *eks.EKS
	ec2Client  *ec2.EC2
	ssmClient  *ssm.SSM
	cfnClient  *cloudformation.CloudFormation
	k8sClient  *kubernetes.Clientset
	s3Client   *s3.S3
	iamClient  *iam.IAM
	cluster    *hybridCluster
	stackIn    *e2eCfnStack
	stackOut   *e2eCfnStackOutput
}

var _ = Describe("Hybrid Nodes", Ordered, func() {
	osList := []NodeadmOS{
		NewUbuntu2004AMD(),
		NewUbuntu2004ARM(),
		NewUbuntu2204AMD(),
		NewUbuntu2204ARM(),
		NewUbuntu2404AMD(),
		NewUbuntu2404ARM(),
		NewAmazonLinux2023AMD(),
		NewAmazonLinux2023ARM(),
		NewRedHat8AMD(os.Getenv("RHEL_USERNAME"), os.Getenv("RHEL_PASSWORD")),
		NewRedHat8ARM(os.Getenv("RHEL_USERNAME"), os.Getenv("RHEL_PASSWORD")),
		NewRedHat9AMD(os.Getenv("RHEL_USERNAME"), os.Getenv("RHEL_PASSWORD")),
		NewRedHat9ARM(os.Getenv("RHEL_USERNAME"), os.Getenv("RHEL_PASSWORD")),
	}

	credentialProviders := []NodeadmCredentialsProvider{&SsmProvider{}}

	When("using peered VPC", func() {
		skipCleanup := os.Getenv("SKIP_CLEANUP") == "true"
		test := &peeredVPCTest{}

		BeforeAll(func(ctx context.Context) {
			awsSession, err := newE2EAWSSession(config.ClusterRegion)
			Expect(err).NotTo(HaveOccurred())

			test.awsSession = awsSession
			test.eksClient = eks.New(awsSession)
			test.ec2Client = ec2.New(awsSession)
			test.ssmClient = ssm.New(awsSession)
			test.s3Client = s3.New(awsSession)
			test.cfnClient = cloudformation.New(awsSession)
			test.iamClient = iam.New(awsSession)

			kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				clientcmd.NewDefaultClientConfigLoadingRules(),
				&clientcmd.ConfigOverrides{},
			)
			clientConfig, err := kubeconfig.ClientConfig()
			Expect(err).NotTo(HaveOccurred(), "expected to load kubeconfig file")

			test.k8sClient, err = kubernetes.NewForConfig(clientConfig)
			Expect(err).NotTo(HaveOccurred(), "expected to build kubernetes client")

			test.cluster, err = getHybridClusterDetails(ctx, test.eksClient, test.ec2Client, config.ClusterName, config.ClusterRegion, config.HybridVpcID)
			Expect(err).NotTo(HaveOccurred(), "expected to get cluster details")

			providerFilter := enabledCredentialsProviders(credentialProviders)
			// if there is no credential provider filter provided, then create resources for all the credential providers
			if len(providerFilter) == 0 {
				providerFilter = credentialProviders
			}
			test.stackIn = &e2eCfnStack{
				clusterName:         test.cluster.clusterName,
				clusterArn:          test.cluster.clusterArn,
				credentialProviders: providerFilter,
				stackName:           fmt.Sprintf("EKSHybridCI-%s-%s", removeSpecialChars(config.ClusterName), getCredentialProviderNames(providerFilter)),
				cfn:                 test.cfnClient,
				iam:                 test.iamClient,
			}
			test.stackOut, err = test.stackIn.deployResourcesStack(ctx, logger)
			Expect(err).NotTo(HaveOccurred(), "e2e resrouce stack should have been deployed")

			for _, provider := range credentialProviders {
				switch provider.Name() {
				case creds.SsmCredentialProvider:
					if ssmProvider, ok := provider.(*SsmProvider); ok {
						ssmProvider.ssmClient = test.ssmClient
						ssmProvider.role = test.stackOut.ssmNodeRoleName
					}
				}
			}
		})

		When("using ec2 instance as hybrid nodes", func() {
			for _, os := range osList {
				for _, provider := range credentialProviders {
					DescribeTable("Joining a node",
						func(ctx context.Context, os NodeadmOS, provider NodeadmCredentialsProvider) {
							Expect(os).NotTo(BeNil())
							Expect(provider).NotTo(BeNil())

							nodeName := fmt.Sprintf("EKSHybridCI-%s", removeSpecialChars(config.ClusterName))
							nodeadmConfig, err := provider.NodeadmConfig(test.cluster)
							Expect(err).NotTo(HaveOccurred(), "expected to build nodeconfig")

							nodeAdmUrls := NodeadmURLs{}
							if config.NodeadmUrlAMD != "" {
								nodeadmUrl, err := getNodeadmURL(test.s3Client, config.NodeadmUrlAMD)
								Expect(err).NotTo(HaveOccurred(), "expected to retrieve nodeadm amd URL from S3 successfully")
								nodeAdmUrls.AMD = nodeadmUrl
							}
							if config.NodeadmUrlARM != "" {
								nodeadmUrl, err := getNodeadmURL(test.s3Client, config.NodeadmUrlARM)
								Expect(err).NotTo(HaveOccurred(), "expected to retrieve nodeadm arm URL from S3 successfully")
								nodeAdmUrls.ARM = nodeadmUrl
							}
							nodeadmConfigYaml, err := yaml.Marshal(&nodeadmConfig)
							Expect(err).NotTo(HaveOccurred(), "expected to successfully marshal nodeadm config to YAML")

							userdata, err := os.BuildUserData(nodeAdmUrls, string(nodeadmConfigYaml), test.cluster.kubernetesVersion, string(provider.Name()))
							Expect(err).NotTo(HaveOccurred(), "expected to successfully build user data")

							amiId, err := os.AMIName(ctx, test.awsSession)
							Expect(err).NotTo(HaveOccurred(), "expected to successfully retrieve ami id")

							ec2Input := ec2InstanceConfig{
								instanceName:    nodeName,
								amiID:           amiId,
								instanceType:    os.InstanceType(),
								volumeSize:      ec2VolumeSize,
								subnetID:        test.cluster.subnetID,
								securityGroupID: test.cluster.securityGroupID,
								userData:        userdata,
								instanceProfile: test.stackOut.ec2InstanceProfile,
							}

							logger.Info("Creating a hybrid ec2 instance...")
							ec2, err := ec2Input.create(ctx, test.ec2Client, test.ssmClient)
							Expect(err).NotTo(HaveOccurred(), "ec2 instance should have been created successfully")

							DeferCleanup(func(ctx context.Context) {
								if skipCleanup {
									logger.Info("Skipping ec2 instance deletion", "instanceID", ec2.instanceID)
									return
								}
								logger.Info("Deleting ec2 instance", "instanceID", ec2.instanceID)
								Expect(deleteEC2Instance(ctx, test.ec2Client, ec2.instanceID)).NotTo(HaveOccurred(), "ec2 instance should have been deleted successfully")
							})
							// get the hybrid node registered using nodeadm by the internal IP of an EC2 instance
							node, err := waitForNode(ctx, test.k8sClient, ec2.ipAddress, logger)
							Expect(err).NotTo(HaveOccurred())
							Expect(node).NotTo(BeNil())
							nodeName = node.Name

							logger.Info("Waiting for hybrid node to be ready...")
							Expect(waitForHybridNodeToBeReady(ctx, test.k8sClient, nodeName, logger)).NotTo(HaveOccurred())

							logger.Info("Creating a test pod on the hybrid node...")
							podName := getNginxPodName(nodeName)
							Expect(createNginxPodInNode(ctx, test.k8sClient, nodeName)).NotTo(HaveOccurred())
							logger.Info(fmt.Sprintf("Pod %s created and running on node %s", podName, nodeName))

							logger.Info("Deleting test pod", "pod", podName)
							Expect(deletePod(ctx, test.k8sClient, podName, podNamespace)).NotTo(HaveOccurred())
							logger.Info("Pod deleted successfully", "pod", podName)

							if skipCleanup {
								logger.Info("Skipping nodeadm uninstall from the hybrid node...")
								return
							}
							logger.Info("Uninstalling nodeadm from the hybrid node...")
							// runNodeadmUninstall takes instanceID as a parameter. Here we are passing nodeName.
							// In case of ssm credential provider, nodeName i.e. "mi-0dddf39dfb164d78a" would be the instanceID.
							// In case of iam ra credential provider, nodeName i.e. "i-0dddf39dfb164d78a" would be the instanceID.
							Expect(runNodeadmUninstall(ctx, test.ssmClient, nodeName, logger)).NotTo(HaveOccurred())

							logger.Info("Deleting hybrid node from the cluster", "hybrid node", nodeName)
							Expect(deleteNode(ctx, test.k8sClient, nodeName)).NotTo(HaveOccurred())
							logger.Info("Node deleted successfully", "node", nodeName)
						},
						Entry(fmt.Sprintf("With OS %s and with Credential Provider %s", os.Name(), string(provider.Name())), context.Background(), os, provider, Label(os.Name(), string(provider.Name()), "simpleflow")),
					)
				}
			}
		})

		AfterAll(func(ctx context.Context) {
			if skipCleanup {
				logger.Info("Skipping cleanup of e2e resources stack")
				return
			}
			logger.Info("Deleting e2e resources stack", "stackName", test.stackIn.stackName)
			err := test.stackIn.deleteResourceStack(ctx, logger)
			Expect(err).NotTo(HaveOccurred(), "failed to delete stack")
		})
	})
})
