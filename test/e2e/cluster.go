package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
)

const createClusterTimeout = 15 * time.Minute

func (t *TestRunner) createEKSCluster(clusterName, kubernetesVersion string) error {
	// Join the subnet IDs into a single comma-separated string
	subnetIdsStr := strings.Join(t.Status.ClusterSubnetIDs, ",")
	hybridNetworkConfig := fmt.Sprintf(`{"remoteNodeNetworks":[{"cidrs":["%s"]}],"remotePodNetworks":[{"cidrs":["%s"]}]}`, t.Spec.HybridNetwork.VpcCidr, t.Spec.HybridNetwork.PodCidr)

	// AWS CLI command to create the cluster
	cmd := exec.Command("aws", "eksbeta", "create-cluster",
		"--name", clusterName,
		"--endpoint-url", fmt.Sprintf("https://eks.%s.amazonaws.com", t.Spec.ClusterRegion),
		"--role-arn", t.Status.RoleArn,
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
func (t *TestRunner) waitForClusterCreation(clusterName string) error {
	svc := eks.New(t.Session)

	ctx, cancel := context.WithTimeout(context.Background(), createClusterTimeout)
	defer cancel()

	statusCh := make(chan string)
	errCh := make(chan error)

	go func() {
		for {
			describeInput := &eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			}
			output, err := svc.DescribeCluster(describeInput)
			if err != nil {
				errCh <- fmt.Errorf("failed to describe cluster %s: %v", clusterName, err)
				return
			}

			clusterStatus := *output.Cluster.Status
			if clusterStatus == eks.ClusterStatusActive {
				statusCh <- clusterStatus
				return
			} else if clusterStatus == eks.ClusterStatusFailed {
				errCh <- fmt.Errorf("cluster %s creation failed", clusterName)
				return
			}

			// Sleep for 30 secs before checking again
			time.Sleep(30 * time.Second)
		}
	}()

	// Wait for the cluster to become active or for the timeout to expire
	select {
	case clusterStatus := <-statusCh:
		fmt.Printf("cluster %s is now %s.\n", clusterName, clusterStatus)
		return nil
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for cluster %s creation", clusterName)
	}
}
