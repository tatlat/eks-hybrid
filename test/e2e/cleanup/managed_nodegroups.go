package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-logr/logr"

	"github.com/aws/eks-hybrid/test/e2e/errors"
)

const (
	deleteNodegroupTimeout = 15 * time.Minute
)

type ManagedNodeGroupCleanup struct {
	eksClient *eks.Client
	logger    logr.Logger
}

func NewManagedNodeGroupCleanup(eksClient *eks.Client, logger logr.Logger) *ManagedNodeGroupCleanup {
	return &ManagedNodeGroupCleanup{
		eksClient: eksClient,
		logger:    logger,
	}
}

type NodeGroupInfo struct {
	ClusterName   string
	NodeGroupName string
}

func (c *ManagedNodeGroupCleanup) ListManagedNodeGroups(ctx context.Context, input FilterInput) ([]NodeGroupInfo, error) {
	// First get list of clusters that match our filter
	paginator := eks.NewListClustersPaginator(c.eksClient, &eks.ListClustersInput{})

	var nodeGroups []NodeGroupInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing EKS clusters: %w", err)
		}

		for _, clusterName := range page.Clusters {
			// Check if cluster matches filter criteria
			clusterInfo, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			})
			if err != nil && errors.IsType(err, &types.ResourceNotFoundException{}) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("describing cluster %s: %w", clusterName, err)
			}

			if !shouldDeleteCluster(clusterInfo.Cluster, input) {
				continue
			}

			// List node groups for this cluster
			ngPaginator := eks.NewListNodegroupsPaginator(c.eksClient, &eks.ListNodegroupsInput{
				ClusterName: aws.String(clusterName),
			})

			for ngPaginator.HasMorePages() {
				ngPage, err := ngPaginator.NextPage(ctx)
				if err != nil {
					return nil, fmt.Errorf("listing node groups for cluster %s: %w", clusterName, err)
				}

				for _, nodeGroupName := range ngPage.Nodegroups {
					// Describe node group to get tags and creation time for filtering
					ngInfo, err := c.eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
						ClusterName:   aws.String(clusterName),
						NodegroupName: aws.String(nodeGroupName),
					})
					if err != nil && errors.IsType(err, &types.ResourceNotFoundException{}) {
						continue
					}
					if err != nil {
						return nil, fmt.Errorf("describing node group %s in cluster %s: %w", nodeGroupName, clusterName, err)
					}

					if shouldDeleteNodeGroup(ngInfo.Nodegroup, input) {
						nodeGroups = append(nodeGroups, NodeGroupInfo{
							ClusterName:   clusterName,
							NodeGroupName: nodeGroupName,
						})
					}
				}
			}
		}
	}

	return nodeGroups, nil
}

func (c *ManagedNodeGroupCleanup) DeleteManagedNodeGroup(ctx context.Context, clusterName, nodeGroupName string) error {
	c.logger.Info("Deleting managed node group", "cluster", clusterName, "nodegroup", nodeGroupName)

	_, err := c.eksClient.DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodeGroupName),
	})

	if err != nil && errors.IsType(err, &types.ResourceNotFoundException{}) {
		c.logger.Info("Node group already deleted", "cluster", clusterName, "nodegroup", nodeGroupName)
		return nil
	}

	if err != nil {
		return fmt.Errorf("deleting node group %s in cluster %s: %w", nodeGroupName, clusterName, err)
	}

	// Wait for node group deletion
	waiter := eks.NewNodegroupDeletedWaiter(c.eksClient)
	err = waiter.Wait(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodeGroupName),
	}, deleteNodegroupTimeout)
	if err != nil {
		return fmt.Errorf("waiting for node group %s deletion in cluster %s: %w", nodeGroupName, clusterName, err)
	}

	c.logger.Info("Successfully deleted managed node group", "cluster", clusterName, "nodegroup", nodeGroupName)
	return nil
}

func shouldDeleteNodeGroup(nodeGroup *types.Nodegroup, input FilterInput) bool {
	var tags []Tag
	for key, value := range nodeGroup.Tags {
		tags = append(tags, Tag{
			Key:   key,
			Value: value,
		})
	}

	resource := ResourceWithTags{
		ID:           *nodeGroup.NodegroupName,
		CreationTime: aws.ToTime(nodeGroup.CreatedAt),
		Tags:         tags,
	}

	return shouldDeleteResource(resource, input)
}
