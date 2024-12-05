package peered

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
)

type HybridCluster struct {
	Name              string
	Arn               string
	Region            string
	KubernetesVersion string
	SubnetID          string
	SecurityGroupID   string
}

func getClusterDetails(ctx context.Context, client *eks.EKS, clusterName string) (eks.Cluster, error) {
	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}

	result, err := client.DescribeClusterWithContext(ctx, input)
	if err != nil {
		return eks.Cluster{}, fmt.Errorf("getting cluster details: %w", err)
	}

	return *result.Cluster, nil
}

func findSubnetInVPC(ctx context.Context, client *ec2.EC2, vpcID string) (subnetID string, err error) {
	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
			{
				Name:   aws.String("map-public-ip-on-launch"),
				Values: []*string{aws.String("true")},
			},
		},
	}
	result, err := client.DescribeSubnetsWithContext(ctx, input)
	if err != nil {
		return "", err
	}

	if len(result.Subnets) == 0 {
		return "", fmt.Errorf("no subnets found in VPC %s", vpcID)
	}
	return *result.Subnets[0].SubnetId, nil
}

func getDefaultSecurityGroup(ctx context.Context, client *ec2.EC2, vpcID string) (string, error) {
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
		},
	}

	result, err := client.DescribeSecurityGroupsWithContext(ctx, input)
	if err != nil {
		return "", err
	}

	if len(result.SecurityGroups) == 0 {
		return "", fmt.Errorf("no default security group found for VPC %s", vpcID)
	}

	return *result.SecurityGroups[0].GroupId, nil
}

func GetHybridCluster(ctx context.Context, eksClient *eks.EKS, ec2Client *ec2.EC2, clusterName, clusterRegion, hybridVpcID string) (*HybridCluster, error) {
	cluster := &HybridCluster{
		Name:   clusterName,
		Region: clusterRegion,
	}

	clusterDetails, err := getClusterDetails(ctx, eksClient, clusterName)
	if err != nil {
		return nil, fmt.Errorf("getting cluster kubernetes version: %w", err)
	}

	cluster.KubernetesVersion = *clusterDetails.Version
	cluster.Arn = *clusterDetails.Arn

	cluster.SubnetID, err = findSubnetInVPC(ctx, ec2Client, hybridVpcID)
	if err != nil {
		return nil, fmt.Errorf("getting public subnet in the given hybrid node vpc %s: %w", hybridVpcID, err)
	}

	cluster.SecurityGroupID, err = getDefaultSecurityGroup(ctx, ec2Client, hybridVpcID)
	if err != nil {
		return nil, fmt.Errorf("getting default security group in the given hybrid node vpc %s: %w", hybridVpcID, err)
	}

	return cluster, nil
}
