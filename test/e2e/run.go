package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"gopkg.in/yaml.v2"
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
	HybridPodCidr      string        `yaml:"hybridPodCidr"`
	KubernetesVersions []string      `yaml:"kubernetesVersions"`
	Cni                []string      `yaml:"cni"`
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

const outputDir = "/tmp/eks-hybrid"

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

var authConfig = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth
  namespace: kube-system
data:
  mapRoles: |
`

func newAWSSession(region string) (*session.Session, error) {
	// Create a new session using shared credentials or environment variables
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new AWS session: %v", err)
	}

	// Optionally, you can log the region for debugging purposes
	fmt.Printf("AWS session initialized in region: %s\n", region)

	return sess, nil
}

func (t *TestRunner) CreateResources() error {
	// Create AWS session
	session, err := newAWSSession(t.Spec.ClusterRegion)
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %v", err)
	}

	t.Session = session

	// Temporary code, remove this when AWS CLI supports creating hybrid EKS cluster
	awsPatchLocation := "s3://eks-hybrid-beta/v0.0.0-beta.1/awscli/aws-eks-cli-beta.json"
	localFilePath := "/tmp/awsclipatch"
	err = patchEksServiceModel(awsPatchLocation, localFilePath)
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
		fmt.Printf("Creating EKS cluster with the kubernetes version %s..", kubernetesVersion)
		clusterName := fmt.Sprintf("%s-%s", t.Spec.ClusterName, strings.Replace(kubernetesVersion, ".", "-", -1))
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

		// Save kubeconfig file for the created cluster under /tmp/eks-hybrid/CULSTERNAME-kubeconfig dir to use it late in e2e test run
		kubeconfigFilePath := filepath.Join(outputDir, fmt.Sprintf("%s.kubeconfig", clusterName))
		err = saveKubeconfig(clusterName, t.Spec.ClusterRegion, kubeconfigFilePath)
		if err != nil {
			return fmt.Errorf("error saving kubeconfig for %s EKS cluster: %v", kubernetesVersion, err)
		}

		fmt.Printf("Kubeconfig saved at: %s\n", kubeconfigFilePath)

		// Patch aws-node DaemonSet to update the VPC CNI with anti-affinity for nodes labeled with the default hybrid nodes label eks.amazonaws.com/compute-type: hybrid
		fmt.Println("Patching aws-node DaemonSet...")
		err = patchAwsNode(kubeconfigFilePath)
		if err != nil {
			return fmt.Errorf("error patching aws-node DaemonSet for %s EKS cluster: %v", kubernetesVersion, err)
		}

		// Apply aws-auth ConfigMap
		fmt.Println("Applying aws-auth ConfigMap...")
		err = applyAwsAuth(kubeconfigFilePath)
		if err != nil {
			return fmt.Errorf("error applying aws-auth ConfigMap for %s EKS cluster: %v", kubernetesVersion, err)
		}
	}

	// After resources are created, write the config to a file
	configFilePath := filepath.Join(outputDir, "setup-resources-output.yaml")
	if err := t.saveSetupConfigAsYAML(configFilePath); err != nil {
		return fmt.Errorf("failed to write config to file: %v", err)
	}

	return nil
}

// saveKubeconfig saves the kubeconfig for the cluster
func saveKubeconfig(clusterName, region, kubeconfigPath string) error {
	cmd := exec.Command("aws", "eks", "update-kubeconfig", "--name", clusterName, "--region", region, "--kubeconfig", kubeconfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// patchAwsNode patches the aws-node DaemonSet
func patchAwsNode(kubeconfig string) error {
	// Patch aws-node using stdin
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "patch", "ds", "aws-node", "-n", "kube-system", "--type", "merge", "--patch-file=/dev/stdin")
	cmd.Stdin = bytes.NewReader([]byte(awsNodePatchContent))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println(cmd)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to patch aws-node DaemonSet: %v", err)
	}

	fmt.Println("Successfully patched aws-node DaemonSet")
	return nil
}

// applyAwsAuth applies the aws-auth configmap
func applyAwsAuth(kubeconfig string) error {
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "apply", "-f", "/dev/stdin")
	cmd.Stdin = bytes.NewReader([]byte(authConfig))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to apply aws-auth ConfigMap: %v", err)
	}

	fmt.Println("Successfully applied aws-auth ConfigMap with empty mapRoles")
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
