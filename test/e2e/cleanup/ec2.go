package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/go-logr/logr"

	"github.com/aws/eks-hybrid/test/e2e/constants"
)

const (
	waitTimeout = 10 * time.Minute
)

type EC2Cleaner struct {
	ec2Client *ec2.Client
	logger    logr.Logger
}

func NewEC2Cleaner(ec2Client *ec2.Client, logger logr.Logger) *EC2Cleaner {
	return &EC2Cleaner{
		ec2Client: ec2Client,
		logger:    logger,
	}
}

func shouldTerminateInstance(instance types.Instance, input FilterInput) bool {
	if instance.State.Name == types.InstanceStateNameTerminated {
		return false
	}

	var tags []Tag
	for _, tag := range instance.Tags {
		tags = append(tags, Tag{
			Key:   *tag.Key,
			Value: *tag.Value,
		})
	}

	resource := ResourceWithTags{
		ID:           *instance.InstanceId,
		CreationTime: aws.ToTime(instance.LaunchTime),
		Tags:         tags,
	}
	return shouldDeleteResource(resource, input)
}

func (e *EC2Cleaner) ListTaggedInstances(ctx context.Context, input FilterInput) ([]string, error) {
	describeInput := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []string{constants.TestClusterTagKey},
			},
		},
	}

	clusterNameFilter := input.ClusterName
	if input.ClusterNamePrefix != "" {
		clusterNameFilter = input.ClusterNamePrefix + "*"
	}
	if clusterNameFilter != "" {
		describeInput.Filters = append(describeInput.Filters, types.Filter{
			Name:   aws.String("tag:" + constants.TestClusterTagKey),
			Values: []string{clusterNameFilter},
		})
	}

	paginator := ec2.NewDescribeInstancesPaginator(e.ec2Client, describeInput)

	var instanceIDs []string

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing instances: %w", err)
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if shouldTerminateInstance(instance, input) {
					instanceIDs = append(instanceIDs, *instance.InstanceId)
				}
			}
		}
	}

	return instanceIDs, nil
}

func (e *EC2Cleaner) DeleteInstances(ctx context.Context, instanceIDs []string) error {
	if len(instanceIDs) == 0 {
		return nil
	}

	_, err := e.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		return fmt.Errorf("terminating instances: %w", err)
	}

	waiter := ec2.NewInstanceTerminatedWaiter(e.ec2Client)
	if err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	}, waitTimeout); err != nil {
		return fmt.Errorf("waiting for instances to terminate: %w", err)
	}

	return nil
}

func (e *EC2Cleaner) ListKeyPairs(ctx context.Context, input FilterInput) ([]string, error) {
	describeInput := &ec2.DescribeKeyPairsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []string{constants.TestClusterTagKey},
			},
		},
	}

	clusterNameFilter := input.ClusterName
	if input.ClusterNamePrefix != "" {
		clusterNameFilter = input.ClusterNamePrefix + "*"
	}
	if clusterNameFilter != "" {
		describeInput.Filters = append(describeInput.Filters, types.Filter{
			Name:   aws.String("tag:" + constants.TestClusterTagKey),
			Values: []string{clusterNameFilter},
		})
	}

	keyPairs, err := e.ec2Client.DescribeKeyPairs(ctx, describeInput)
	if err != nil {
		return nil, fmt.Errorf("describing key pairs: %w", err)
	}

	var keyPairIDs []string
	for _, keyPair := range keyPairs.KeyPairs {
		if shouldDeleteKeyPair(keyPair, input) {
			keyPairIDs = append(keyPairIDs, *keyPair.KeyPairId)
		}
	}

	return keyPairIDs, nil
}

func (e *EC2Cleaner) DeleteKeyPair(ctx context.Context, keyPairID string) error {
	_, err := e.ec2Client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
		KeyPairId: aws.String(keyPairID),
	})
	if err != nil {
		return fmt.Errorf("deleting key pair: %w", err)
	}
	return nil
}

func shouldDeleteKeyPair(keyPair types.KeyPairInfo, input FilterInput) bool {
	var tags []Tag
	for _, tag := range keyPair.Tags {
		tags = append(tags, Tag{
			Key:   *tag.Key,
			Value: *tag.Value,
		})
	}

	resource := ResourceWithTags{
		ID:           *keyPair.KeyPairId,
		CreationTime: aws.ToTime(keyPair.CreateTime),
		Tags:         tags,
	}
	return shouldDeleteResource(resource, input)
}
