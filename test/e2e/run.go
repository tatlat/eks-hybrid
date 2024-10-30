package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	sigyaml "sigs.k8s.io/yaml"
)

type TestRunner struct {
	Session *session.Session   `yaml:"-"`
	Spec    TestResourceSpec   `yaml:"spec"`
	Status  TestResourceStatus `yaml:"status"`
}

type TestResourceSpec struct {
	ClusterName        string        `yaml:"clusterName"`
	ClusterRegion      string        `yaml:"clusterRegion"`
	ClusterNetwork     NetworkConfig `yaml:"clusterNetwork"`
	HybridNetwork      NetworkConfig `yaml:"hybridNetwork"`
	KubernetesVersions []string      `yaml:"kubernetesVersions"`
	Cni                string        `yaml:"cni"`
}

type TestResourceStatus struct {
	ClusterVpcID     string   `yaml:"clusterVpcID"`
	ClusterSubnetIDs []string `yaml:"clusterSubnetIDs"`
	HybridVpcID      string   `yaml:"hybridVpcID"`
	HybridSubnetIDs  []string `yaml:"hybridSubnetIDs"`
	PeeringConnID    string   `yaml:"peeringConnID"`
	RoleArn          string   `yaml:"roleArn"`
}

type NetworkConfig struct {
	VpcCidr           string `yaml:"vpcCidr"`
	PrivateSubnetCidr string `yaml:"privateSubnetCidr"`
	PublicSubnetCidr  string `yaml:"publicSubnetCidr"`
	PodCidr           string `yaml:"podCidr"`
}

const (
	vpcCNIDaemonSetName = "aws-node"
	vpcCNIDaemonSetNS   = "kube-system"
	outputDir           = "/tmp/eks-hybrid"
	ciliumCni           = "cilium"
)

var awsNodePatchContent = `
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/os
                operator: In
                values:
                - "linux"
              - key: kubernetes.io/arch
                operator: In
                values:
                - "amd64"
                - "arm64"
              - key: eks.amazonaws.com/compute-type
                operator: NotIn
                values:
                - "hybrid"
`

func (t *TestRunner) NewAWSSession() (*session.Session, error) {
	// Create a new session using shared credentials or environment variables
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(t.Spec.ClusterRegion),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new AWS session: %v", err)
	}

	// Optionally, you can log the region for debugging purposes
	fmt.Printf("AWS session initialized in region: %s\n", t.Spec.ClusterRegion)

	return sess, nil
}

func (t *TestRunner) CreateResources(ctx context.Context) error {
	// Temporary code, remove this when AWS CLI supports creating hybrid EKS cluster
	awsPatchLocation := "s3://eks-hybrid-beta/v0.0.0-beta.1/awscli/aws-eks-cli-beta.json"
	localFilePath := "/tmp/awsclipatch"
	err := patchEksServiceModel(awsPatchLocation, localFilePath)
	if err != nil {
		return fmt.Errorf("failed to download aws cli patch that contains hybrid nodes API: %v", err)
	}

	// Create EKS cluster role
	fmt.Println("Creating EKS cluster IAM Role...")
	err = t.createEKSClusterRole()
	if err != nil {
		return fmt.Errorf("error creating IAM role: %v", err)
	}

	// Create EKS cluster VPC
	clusterVpcParam := vpcSubnetParams{
		vpcName:           fmt.Sprintf("%s-vpc", t.Spec.ClusterName),
		vpcCidr:           t.Spec.ClusterNetwork.VpcCidr,
		publicSubnetCidr:  t.Spec.ClusterNetwork.PublicSubnetCidr,
		privateSubnetCidr: t.Spec.ClusterNetwork.PrivateSubnetCidr,
	}
	fmt.Println("Creating EKS cluster VPC...")
	clusterVpcConfig, err := t.createVPC(clusterVpcParam)
	if err != nil {
		return fmt.Errorf("error creating cluster VPC: %v", err)
	}
	t.Status.ClusterVpcID = clusterVpcConfig.vpcID
	t.Status.ClusterSubnetIDs = clusterVpcConfig.subnetIDs

	// Create hybrid nodes VPC
	hybridNodesVpcParam := vpcSubnetParams{
		vpcName:           fmt.Sprintf("%s-hybrid-node-vpc", t.Spec.ClusterName),
		vpcCidr:           t.Spec.HybridNetwork.VpcCidr,
		publicSubnetCidr:  t.Spec.HybridNetwork.PublicSubnetCidr,
		privateSubnetCidr: t.Spec.HybridNetwork.PrivateSubnetCidr,
	}
	fmt.Println("Creating EC2 hybrid nodes VPC...")
	hybridNodesVpcConfig, err := t.createVPC(hybridNodesVpcParam)
	if err != nil {
		return fmt.Errorf("error creating EC2 hybrid nodes VPC: %v", err)
	}
	t.Status.HybridVpcID = hybridNodesVpcConfig.vpcID
	t.Status.HybridSubnetIDs = hybridNodesVpcConfig.subnetIDs

	// Create VPC Peering Connection between the cluster VPC and EC2 hybrid nodes VPC
	fmt.Println("Creating VPC peering connection...")
	t.Status.PeeringConnID, err = t.createVPCPeering()
	if err != nil {
		return fmt.Errorf("error creating VPC peering connection: %v", err)
	}

	// Update route tables for peering connection
	fmt.Println("Updating route tables for VPC peering...")
	err = t.updateRouteTablesForPeering()
	if err != nil {
		return fmt.Errorf("error updating route tables: %v", err)
	}

	// Create the EKS Cluster using the IAM role and VPC
	for _, kubernetesVersion := range t.Spec.KubernetesVersions {
		fmt.Printf("Creating EKS cluster with the kubernetes version %s..\n", kubernetesVersion)
		clusterName := clusterName(t.Spec.ClusterName, kubernetesVersion)
		err := t.createEKSCluster(clusterName, kubernetesVersion)
		if err != nil {
			return fmt.Errorf("error creating %s EKS cluster: %v", kubernetesVersion, err)
		}

		fmt.Printf("Cluster creation of kubernetes version %s process started successfully..", kubernetesVersion)

		// Wait for the cluster to be ready
		err = t.waitForClusterCreation(clusterName)
		if err != nil {
			return fmt.Errorf("error while waiting for cluster creation: %v", err)
		}

		err = updateKubeconfig(clusterName, t.Spec.ClusterRegion)
		if err != nil {
			return fmt.Errorf("error saving kubeconfig for %s EKS cluster: %v", kubernetesVersion, err)
		}

		kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)
		clientConfig, err := kubeconfig.ClientConfig()
		if err != nil {
			return fmt.Errorf("error loading kubeconfig: %v", err)
		}

		k8sClient, err := kubernetes.NewForConfig(clientConfig)
		if err != nil {
			return fmt.Errorf("error creating Kubernetes client: %v", err)
		}

		dynamicK8s, err := dynamic.NewForConfig(clientConfig)
		if err != nil {
			return fmt.Errorf("error creating dynamic Kubernetes client: %v", err)
		}

		// Patch aws-node DaemonSet to update the VPC CNI with anti-affinity for nodes labeled with the default hybrid nodes label eks.amazonaws.com/compute-type: hybrid
		fmt.Println("Patching aws-node daemonSet...")
		err = patchAwsNode(ctx, k8sClient)
		if err != nil {
			return fmt.Errorf("error patching aws-node daemonSet for %s EKS cluster: %v", kubernetesVersion, err)
		}

		switch t.Spec.Cni {
		case ciliumCni:
			cilium := newCilium(dynamicK8s, t.Spec.HybridNetwork.PodCidr)
			fmt.Printf("Installing cilium on cluster %s...\n", clusterName)
			if err = cilium.deploy(ctx); err != nil {
				return fmt.Errorf("error installing cilium for %s EKS cluster: %v", kubernetesVersion, err)
			}
			fmt.Println("Cilium installed sucessfully.")
		}
	}

	// After resources are created, write the config to a file
	configFilePath := filepath.Join(outputDir, "setup-resources-output.yaml")
	if err := t.saveSetupConfigAsYAML(configFilePath); err != nil {
		return fmt.Errorf("failed to write config to file: %v", err)
	}

	return nil
}

func clusterName(clusterName, kubernetesVersion string) string {
	return fmt.Sprintf("%s-%s", clusterName, replaceDotsWithDashes(kubernetesVersion))
}

// saveKubeconfig saves the kubeconfig for the cluster
func updateKubeconfig(clusterName, region string) error {
	cmd := exec.Command("aws", "eks", "update-kubeconfig", "--name", clusterName, "--region", region)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func patchAwsNode(ctx context.Context, k8s *kubernetes.Clientset) error {
	patchJSON, err := sigyaml.YAMLToJSON([]byte(awsNodePatchContent))
	if err != nil {
		return fmt.Errorf("error marshalling patch data: %v", err)
	}

	_, err = k8s.AppsV1().DaemonSets(vpcCNIDaemonSetNS).Patch(
		ctx,
		vpcCNIDaemonSetName,
		types.StrategicMergePatchType,
		patchJSON,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("error patching %s daemonSet: %v", vpcCNIDaemonSetName, err)
	}

	fmt.Println("Successfully patched aws-node daemonSet")
	return nil
}

func (t *TestRunner) saveSetupConfigAsYAML(outputFile string) error {
	testRunnerContent, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("error marshaling test runner config: %v", err)
	}
	if err = os.WriteFile(outputFile, testRunnerContent, 0o644); err != nil {
		return err
	}

	fmt.Printf("Successfully saved resource configuration to %s\n", outputFile)
	return nil
}

// replaceDotsWithDashes replaces dots in the Kubernetes version with dashes
func replaceDotsWithDashes(version string) string {
	return strings.Replace(version, ".", "-", -1)
}
