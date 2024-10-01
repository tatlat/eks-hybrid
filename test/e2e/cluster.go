package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/eks"
)

const (
	createClusterTimeout = 15 * time.Minute
	deleteClusterTimeout = 5 * time.Minute
)

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

// cleanupEKSHybridClusters cleans up all the clusters for all the kubernetes version.
func (t *TestRunner) cleanupEKSHybridClusters(ctx context.Context) error {
	for _, kubernetesVersion := range t.Spec.KubernetesVersions {
		clusterName := clusterName(t.Spec.ClusterName, kubernetesVersion)

		fmt.Printf("cleaning up EKS hybrid cluster: %s\n", clusterName)
		err := t.deleteEKSCluster(ctx, clusterName)
		if err != nil {
			fmt.Printf("error cleaning up EKS hybrid cluster %s: %v\n", clusterName, err)
			return err
		}
		fmt.Printf("successfully deleted EKS hybrid cluster: %s\n", clusterName)
	}
	return nil
}

// deleteEKSCluster deletes the given EKS cluster
func (t *TestRunner) deleteEKSCluster(ctx context.Context, clusterName string) error {
	svc := eks.New(t.Session)
	fmt.Printf("deleting cluster %s\n", clusterName)
	_, err := svc.DeleteCluster(&eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete EKS hybrid cluster %s: %v", clusterName, err)
	}

	fmt.Printf("cluster deletion initiated for: %s\n", clusterName)

	// Wait for the cluster to be fully deleted to check for any errors during the delete.
	err = waitForClusterDeletion(ctx, svc, clusterName)
	if err != nil {
		return fmt.Errorf("error waiting for cluster %s deletion: %v", clusterName, err)
	}

	return nil
}

// waitForClusterDeletion waits for the cluster to be deleted.
func waitForClusterDeletion(ctx context.Context, svc *eks.EKS, clusterName string) error {
	// Create a context that automatically cancels after the specified timeout
	ctx, cancel := context.WithTimeout(ctx, deleteClusterTimeout)
	defer cancel()

	statusCh := make(chan bool)
	errCh := make(chan error)

	go func(ctx context.Context) {
		defer close(statusCh)
		defer close(errCh)
		for {
			select {
			case <-ctx.Done(): // Check if the context is done (timeout/canceled)
				errCh <- fmt.Errorf("context canceled or timed out while waiting for cluster %s deletion: %v", clusterName, ctx.Err())
				return
			case <-time.After(30 * time.Second): // Retry after 30 secs
				// Continue checking for deletion
			default:
				describeInput := &eks.DescribeClusterInput{
					Name: aws.String(clusterName),
				}
				_, err := svc.DescribeCluster(describeInput)
				if err != nil {
					if isClusterNotFoundError(err) {
						statusCh <- true
						return
					}
					errCh <- fmt.Errorf("failed to describe cluster %s: %v", clusterName, err)
					return
				}
			}
		}
	}(ctx)

	// Wait for the cluster to be deleted or for the timeout to expire
	select {
	case <-statusCh:
		fmt.Printf("cluster %s successfully deleted.\n", clusterName)
		return nil
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for cluster %s deletion", clusterName)
	}
}

// isClusterNotFoundError checks if the error is due to the cluster not being found
func isClusterNotFoundError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceNotFoundException
	}
	return false
}
