package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/service/eks"

	ekscustom "github.com/aws/eks-hybrid/test/e2e/eks"
)

const (
	createClusterTimeout = 15 * time.Minute
	deleteClusterTimeout = 5 * time.Minute
)

func (t *TestRunner) createEKSCluster(ctx context.Context, clusterName, kubernetesVersion, clusterSecurityGroupID string) error {
	svc := eks.New(t.Session)
	eksCluster := &ekscustom.CreateClusterInput{
		Name:    aws.String(clusterName),
		Version: aws.String(kubernetesVersion),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			SubnetIds:        aws.StringSlice(t.Status.ClusterSubnetIDs),
			SecurityGroupIds: aws.StringSlice([]string{clusterSecurityGroupID}),
		},
		RoleArn: aws.String(t.Status.RoleArn),
		Tags: map[string]*string{
			"Name":            aws.String(fmt.Sprintf("%s-%s", clusterName, kubernetesVersion)),
			TestClusterTagKey: aws.String(clusterName),
		},
		AccessConfig: &eks.CreateAccessConfigRequest{
			AuthenticationMode: aws.String("API_AND_CONFIG_MAP"),
		},
		RemoteNetworkConfig: &ekscustom.RemoteNetworkConfig{
			RemoteNodeNetworks: []*ekscustom.RemoteNodeNetwork{
				{
					CIDRs: aws.StringSlice([]string{t.Spec.HybridNetwork.VpcCidr}),
				},
			},
			RemotePodNetworks: []*ekscustom.RemotePodNetwork{
				{
					CIDRs: aws.StringSlice([]string{t.Spec.HybridNetwork.PodCidr}),
				},
			},
		},
	}
	clusterOutput, err := ekscustom.CreateCluster(ctx, svc, eksCluster)
	if err != nil && !isErrCode(err, eks.ErrCodeResourceInUseException) {
		return fmt.Errorf("creating EKS hybrid cluster: %w", err)
	}

	if clusterOutput.Cluster != nil {
		fmt.Printf("Successfully started EKS hybrid cluster: %s\nOutput: %s\n", clusterName, awsutil.Prettify(clusterOutput))
	}

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

// deleteEKSCluster deletes the given EKS cluster
func (t *TestRunner) deleteEKSCluster(ctx context.Context, clusterName string) error {
	svc := eks.New(t.Session)
	_, err := svc.DeleteCluster(&eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil && isErrCode(err, eks.ErrCodeResourceNotFoundException) {
		fmt.Printf("Cluster %s already deleted\n", clusterName)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to delete EKS hybrid cluster %s: %v", clusterName, err)
	}

	fmt.Printf("Cluster deletion initiated for: %s\n", clusterName)

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
			select {
			case <-ctx.Done(): // Check if the context is done (timeout/canceled)
				errCh <- fmt.Errorf("context canceled or timed out while waiting for cluster %s deletion: %v", clusterName, ctx.Err())
				return
			case <-time.After(30 * time.Second): // Retry after 30 secs
				fmt.Printf("Waiting for cluster %s to be deleted.\n", clusterName)
			}
		}
	}(ctx)

	// Wait for the cluster to be deleted or for the timeout to expire
	select {
	case <-statusCh:
		fmt.Printf("Cluster %s successfully deleted.\n", clusterName)
		return nil
	case err := <-errCh:
		return err
	}
}

// isClusterNotFoundError checks if the error is due to the cluster not being found
func isClusterNotFoundError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceNotFoundException
	}
	return false
}
