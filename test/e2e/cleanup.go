package e2e

import (
	"context"
	"fmt"
)

func (t *TestRunner) CleanupE2EResources(ctx context.Context) error {
	fmt.Printf("Cleaning up EKS hybrid cluster: %s\n", t.Spec.ClusterName)
	err := t.deleteEKSCluster(ctx, t.Spec.ClusterName)
	if err != nil {
		return fmt.Errorf("deleting EKS hybrid cluster: %v", err)
	}

	// Clean up VPC peering connections
	if err := t.deleteVpcPeering(); err != nil {
		return fmt.Errorf("failed to clean up VPC peering: %v", err)
	}

	// Clean up VPCs
	clusterVpcConfig := vpcConfig{
		vpcID:     t.Status.ClusterVpcID,
		subnetIDs: t.Status.ClusterSubnetIDs,
	}
	if err := t.deleteVpc(clusterVpcConfig); err != nil {
		return fmt.Errorf("failed to clean up cluster VPC: %v", err)
	}

	hybridVpcConfig := vpcConfig{
		vpcID:     t.Status.HybridVpcID,
		subnetIDs: t.Status.HybridSubnetIDs,
	}
	if err := t.deleteVpc(hybridVpcConfig); err != nil {
		return fmt.Errorf("failed to clean up EC2 VPC: %v", err)
	}

	// Clean up IAM role created for EKS hybrid cluster.
	if err := t.deleteIamRole(); err != nil {
		return fmt.Errorf("failed to clean up IAM roles: %v", err)
	}

	fmt.Println("Cleanup completed successfully!")
	return nil
}
