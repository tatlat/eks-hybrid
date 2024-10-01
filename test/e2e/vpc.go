package e2e

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// vpcConfig holds information about the VPC and its subnets
type vpcConfig struct {
	vpcID     string
	subnetIDs []string
}

// vpcSubnetParams holds information about the VPC params
type vpcSubnetParams struct {
	vpcName           string
	vpcCidr           string
	publicSubnetCidr  string
	privateSubnetCidr string
}

// CreateVPC creates a VPC and the associated subnets (public and private)
func (t *TestRunner) createVPC(vpcSubnetParams vpcSubnetParams) (*vpcConfig, error) {
	svc := ec2.New(t.Session)

	// Create VPC
	vpcOutput, err := svc.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: aws.String(vpcSubnetParams.vpcCidr),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %v", err)
	}

	vpcId := *vpcOutput.Vpc.VpcId

	// Tag the VPC
	_, err = svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{&vpcId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(vpcSubnetParams.vpcName),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to tag VPC: %v", err)
	}

	fmt.Printf("Successfully created VPC: %s\n", vpcId)

	// Create a public subnet
	publicSubnet, err := svc.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcId),
		CidrBlock:        aws.String(vpcSubnetParams.publicSubnetCidr),
		AvailabilityZone: aws.String(t.Spec.ClusterRegion + "a"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create public subnet: %v", err)
	}

	publicSubnetId := *publicSubnet.Subnet.SubnetId
	fmt.Printf("Successfully created public subnet: %s\n", publicSubnetId)

	// Tag the public subnet
	_, err = svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{&publicSubnetId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(vpcSubnetParams.vpcName + "-public-subnet"),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to tag public subnet: %v", err)
	}

	// Create a private subnet
	privateSubnet, err := svc.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcId),
		CidrBlock:        aws.String(vpcSubnetParams.privateSubnetCidr),
		AvailabilityZone: aws.String(t.Spec.ClusterRegion + "b"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create private subnet: %v", err)
	}

	privateSubnetId := *privateSubnet.Subnet.SubnetId
	fmt.Printf("Successfully created private subnet: %s\n", privateSubnetId)

	// Tag the private subnet
	_, err = svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{&privateSubnetId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(vpcSubnetParams.vpcName + "-private-subnet"),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to tag private subnet: %v", err)
	}

	// Return VPC and subnet information
	vpcConfig := &vpcConfig{
		vpcID:     vpcId,
		subnetIDs: []string{publicSubnetId, privateSubnetId},
	}

	return vpcConfig, nil
}

// CreateVPCPeering creates a VPC peering connection between two VPCs
func (t *TestRunner) createVPCPeering() (string, error) {
	svc := ec2.New(t.Session)

	// Create VPC peering connection
	result, err := svc.CreateVpcPeeringConnection(&ec2.CreateVpcPeeringConnectionInput{
		VpcId:     aws.String(t.Status.ClusterVpcID),
		PeerVpcId: aws.String(t.Status.HybridVpcID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create VPC peering connection: %v", err)
	}

	peeringConnectionID := *result.VpcPeeringConnection.VpcPeeringConnectionId

	fmt.Printf("VPC Peering Connection created: %s\n", peeringConnectionID)

	// Accept the VPC peering request
	_, err = svc.AcceptVpcPeeringConnection(&ec2.AcceptVpcPeeringConnectionInput{
		VpcPeeringConnectionId: aws.String(peeringConnectionID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to accept VPC peering connection: %v", err)
	}

	fmt.Printf("VPC Peering Connection accepted: %s\n", peeringConnectionID)
	return peeringConnectionID, nil
}

// UpdateRouteTablesForPeering updates the route tables to allow traffic between peered VPCs
func (t *TestRunner) updateRouteTablesForPeering() error {
	svc := ec2.New(t.Session)

	// Get the route tables for both VPCs
	routeTables1, err := svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(t.Status.ClusterVpcID)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe route tables for VPC %s: %v", t.Status.ClusterVpcID, err)
	}

	routeTables2, err := svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(t.Status.HybridVpcID)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe route tables for VPC %s: %v", t.Status.HybridVpcID, err)
	}

	// Add routes to the route tables for both VPCs
	for _, rt := range routeTables1.RouteTables {
		_, err = svc.CreateRoute(&ec2.CreateRouteInput{
			RouteTableId:           rt.RouteTableId,
			DestinationCidrBlock:   aws.String(t.Spec.HybridNetwork.VpcCidr),
			VpcPeeringConnectionId: aws.String(t.Status.PeeringConnID),
		})
		if err != nil {
			return fmt.Errorf("failed to create route in VPC %s: %v", t.Status.ClusterVpcID, err)
		}
	}

	for _, rt := range routeTables2.RouteTables {
		_, err = svc.CreateRoute(&ec2.CreateRouteInput{
			RouteTableId:           rt.RouteTableId,
			DestinationCidrBlock:   aws.String(t.Spec.ClusterNetwork.VpcCidr),
			VpcPeeringConnectionId: aws.String(t.Status.PeeringConnID),
		})
		if err != nil {
			return fmt.Errorf("failed to create route in VPC %s: %v", t.Status.HybridVpcID, err)
		}
	}

	fmt.Println("Routes updated for VPC peering connection")
	return nil
}

// deleteVpcPeering deletes the VPC peering connections.
func (t *TestRunner) deleteVpcPeering() error {
	fmt.Println("deleting VPC peering connection...")

	svc := ec2.New(t.Session)
	input := &ec2.DeleteVpcPeeringConnectionInput{
		VpcPeeringConnectionId: aws.String(t.Status.PeeringConnID),
	}

	_, err := svc.DeleteVpcPeeringConnection(input)
	if err != nil {
		return fmt.Errorf("failed to delete VPC peering connection: %v", err)
	}

	fmt.Println("successfully deleted VPC peering connection")
	return nil
}

// deleteVpcs deletes the VPC and their associated subnets.
func (t *TestRunner) deleteVpc(vpc vpcConfig) error {
	fmt.Printf("Deleting VPC %s...\n", vpc.vpcID)

	svc := ec2.New(t.Session)
	// Delete subnets in the VPC
	for _, subnetID := range vpc.subnetIDs {
		err := t.deleteSubnet(subnetID)
		if err != nil {
			return fmt.Errorf("failed to delete subnet %s: %v", subnetID, err)
		}
	}

	// Delete the VPC
	input := &ec2.DeleteVpcInput{
		VpcId: aws.String(vpc.vpcID),
	}

	_, err := svc.DeleteVpc(input)
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %v", err)
	}

	fmt.Printf("Successfully deleted VPC %s\n", vpc.vpcID)
	return nil
}

func (t *TestRunner) deleteSubnet(subnetID string) error {
	fmt.Printf("Deleting subnet %s...\n", subnetID)

	svc := ec2.New(t.Session)
	input := &ec2.DeleteSubnetInput{
		SubnetId: aws.String(subnetID),
	}

	_, err := svc.DeleteSubnet(input)
	if err != nil {
		return fmt.Errorf("failed to delete subnet: %v", err)
	}

	fmt.Printf("Successfully deleted subnet %s\n", subnetID)
	return nil
}
