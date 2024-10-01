package e2e

import (
	"context"
	"fmt"
)

func (t *TestRunner) CleanupE2EResources(ctx context.Context) error {
	if err := t.cleanupEKSHybridClusters(ctx); err != nil {
		return fmt.Errorf("failed to delete EKS hybrid cluster: %v", err)
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

	fmt.Println("cleanup completed successfully!")
	return nil
}
