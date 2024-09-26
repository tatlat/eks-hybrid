package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
)

func (config *ClusterConfig) createEKSCluster(clusterName, kubernetesVersion string) error {
	// Join the subnet IDs into a single comma-separated string
	subnetIdsStr := strings.Join(config.ClusterSubnetIDs, ",")
	hybridNetworkConfig := fmt.Sprintf(`{"remoteNodeNetworks":[{"cidrs":["%s"]}],"remotePodNetworks":[{"cidrs":["%s"]}]}`, config.HybridNodeCidr, config.HybridPodCidr)

	// AWS CLI command to create the cluster
	cmd := exec.Command("aws", "eksbeta", "create-cluster",
		"--name", clusterName,
		"--endpoint-url", fmt.Sprintf("https://eks.%s.amazonaws.com", config.ClusterRegion),
		"--role-arn", config.RoleArn,
		"--resources-vpc-config", fmt.Sprintf("subnetIds=%s", subnetIdsStr),
		"--remote-network-config", hybridNetworkConfig,
		"--access-config", "authenticationMode=API_AND_CONFIG_MAP",
		"--tags", "Name=hybrid-eks-cluster,App=hybrid-eks-beta")

	// Run the command and get the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("failed to create EKS cluster: %v\nOutput: %s", err, string(output))
	}

	fmt.Printf("Successfully started creating EKS cluster: %s\nOutput: %s", clusterName, string(output))
	return nil
}

// waitForClusterCreation waits until the EKS cluster is in the 'ACTIVE' state.
func (config *ClusterConfig) waitForClusterCreation(clusterName string) error {
	svc := eks.New(config.Session)

	fmt.Printf("Waiting for cluster %s to be created...\n", clusterName)

	// Poll the cluster status every 30 secs
	for {
		input := &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		}

		result, err := svc.DescribeCluster(input)
		if err != nil {
			return fmt.Errorf("failed to describe cluster: %v", err)
		}

		clusterStatus := *result.Cluster.Status
		if clusterStatus == eks.ClusterStatusActive {
			fmt.Printf("Cluster %s is now active.\n", clusterName)
			break
		}

		fmt.Printf("Cluster status: %s. Waiting...\n", clusterStatus)
		time.Sleep(30 * time.Second) // Wait for 30 seconds before checking again
	}

	return nil
}
