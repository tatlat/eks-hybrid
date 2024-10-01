package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
)

type ClusterConfig struct {
	Session                  *session.Session
	ClusterName              string
	ClusterVpcCidr           string
	ClusterPrivateSubnetCidr string
	ClusterPublicSubnetCidr  string
	ClusterRegion            string
	PatchLocation            string
	HybridNodeCidr           string
	HybridPubicSubnetCidr    string
	HybridPrivateSubnetCidr  string
	HybridPodCidr            string
	KubernetesVersions       []string
	Networking               []string
	RoleArn                  string
	ClusterVpcID             string
	ClusterSubnetIDs         []string
	HybridVpcID              string
	PeeringConnID            string
}

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

func CreateResources(config *ClusterConfig) error {
	// Temporary code, remove this when AWS CLI supports creating hybrid EKS cluster
	awsPatchLocation := "s3://eks-hybrid-beta/v0.0.0-beta.1/awscli/aws-eks-cli-beta.json"
	localFilePath := "/tmp/awsclipatch"
	err := patchEksServiceModel(awsPatchLocation, localFilePath)
	if err != nil {
		return fmt.Errorf("failed to download aws cli patch that contains hybrid nodes API: %v", err)
	}

	// Create EKS cluster role
	fmt.Println("Creating EKS cluster IAM Role...")
	config.RoleArn, err = config.createEKSClusterRole()
	if err != nil {
		return fmt.Errorf("error creating IAM role: %v", err)
	}

	// Create EKS cluster VPC
	clusterVpcParam := vpcSubnetParams{
		vpcName:           fmt.Sprintf("%s-vpc", config.ClusterName),
		vpcCidr:           config.ClusterVpcCidr,
		publicSubnetCidr:  config.ClusterPublicSubnetCidr,
		privateSubnetCidr: config.ClusterPrivateSubnetCidr,
	}
	fmt.Println("Creating EKS cluster VPC...")
	clusterVpcConfig, err := config.createVPC(clusterVpcParam)
	if err != nil {
		return fmt.Errorf("error creating cluster VPC: %v", err)
	}
	config.ClusterVpcID = clusterVpcConfig.vpcID
	config.ClusterSubnetIDs = clusterVpcConfig.subnetIDs

	// Create hybrid nodes VPC
	hybridNodesVpcParam := vpcSubnetParams{
		vpcName:           fmt.Sprintf("%s-hybrid-node-vpc", config.ClusterName),
		vpcCidr:           config.HybridNodeCidr,
		publicSubnetCidr:  config.HybridPubicSubnetCidr,
		privateSubnetCidr: config.HybridPrivateSubnetCidr,
	}
	fmt.Println("Creating EC2 hybrid nodes VPC...")
	hybridNodesVpcConfig, err := config.createVPC(hybridNodesVpcParam)
	if err != nil {
		return fmt.Errorf("error creating EC2 hybrid nodes VPC: %v", err)
	}
	config.HybridVpcID = hybridNodesVpcConfig.vpcID

	// Create VPC Peering Connection between the cluster VPC and EC2 hybrid nodes VPC
	fmt.Println("Creating VPC peering connection...")
	config.PeeringConnID, err = config.createVPCPeering()
	if err != nil {
		return fmt.Errorf("error creating VPC peering connection: %v", err)
	}

	// Update route tables for peering connection
	fmt.Println("Updating route tables for VPC peering...")
	err = config.updateRouteTablesForPeering()
	if err != nil {
		return fmt.Errorf("error updating route tables: %v", err)
	}

	// Create the EKS Cluster using the IAM role and VPC
	for _, kubernetesVersion := range config.KubernetesVersions {
		fmt.Printf("Creating EKS cluster with the kubernetes version %s..", kubernetesVersion)
		clusterName := fmt.Sprintf("%s-%s", config.ClusterName, strings.Replace(kubernetesVersion, ".", "-", -1))
		err := config.createEKSCluster(clusterName, kubernetesVersion)
		if err != nil {
			return fmt.Errorf("error creating %s EKS cluster: %v", kubernetesVersion, err)
		}

		fmt.Printf("Cluster creation of kubernetes version %s process started successfully..", kubernetesVersion)

		// Wait for the cluster to be ready
		err = config.waitForClusterCreation(clusterName)
		if err != nil {
			return fmt.Errorf("error while waiting for cluster creation: %v", err)
		}

		// Save kubeconfig file for the created cluster under /tmp/eks-hybrid/CULSTERNAME-kubeconfig dir to use it late in e2e test run
		kubeconfigFilePath := filepath.Join("/tmp/eks-hybrid", fmt.Sprintf("%s.kubeconfig", clusterName))
		err = saveKubeconfig(clusterName, config.ClusterRegion, kubeconfigFilePath)
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
