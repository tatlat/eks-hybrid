package peered

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/eks-hybrid/test/e2e/constants"
)

const waitTimeout = 10 * time.Minute

// JumpboxInstance returns the jumpbox ec2 instance for the given cluster.
func JumpboxInstance(ctx context.Context, client *ec2.Client, clusterName string) (*types.Instance, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + constants.TestClusterTagKey),
				Values: []string{clusterName},
			},
			{
				Name:   aws.String("tag:Jumpbox"),
				Values: []string{"true"},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"pending", "running"},
			},
		},
	}
	waiter := ec2.NewInstanceRunningWaiter(client, func(isowo *ec2.InstanceRunningWaiterOptions) {
		isowo.MinDelay = 5 * time.Second
	})
	instances, err := waiter.WaitForOutput(ctx, input, waitTimeout)
	if err != nil {
		return nil, fmt.Errorf("waiting for jumpbox instance: %w", err)
	}
	if len(instances.Reservations) == 0 || len(instances.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("no jumpbox instance found for cluster %s", clusterName)
	}

	return &instances.Reservations[0].Instances[0], nil
}
