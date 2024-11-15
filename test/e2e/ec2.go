//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// instanceConfig holds the configuration for the EC2 instance.
type ec2InstanceConfig struct {
	instanceName    string
	amiID           string
	instanceType    string
	instanceProfile string
	volumeSize      int64
	userData        []byte
	subnetID        string
	securityGroupID string
}

type ec2Instance struct {
	instanceID string
	ipAddress  string
}

func (e *ec2InstanceConfig) create(ctx context.Context, ec2Client *ec2.EC2, ssmClient *ssm.SSM) (ec2Instance, error) {
	userDataEncoded := base64.StdEncoding.EncodeToString(e.userData)

	runResult, err := ec2Client.RunInstances(&ec2.RunInstancesInput{
		ImageId:      aws.String(e.amiID),
		InstanceType: aws.String(e.instanceType),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: aws.String(e.instanceProfile),
		},
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize: aws.Int64(e.volumeSize),
				},
			},
		},
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex: aws.Int64(0),
				SubnetId:    aws.String(e.subnetID),
				Groups:      []*string{aws.String(e.securityGroupID)},
			},
		},
		UserData: aws.String(userDataEncoded),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(e.instanceName),
					},
				},
			},
		},
	})
	if err != nil {
		return ec2Instance{}, fmt.Errorf("could not create hybrid EC2 instance: %w", err)
	}

	return ec2Instance{instanceID: *runResult.Instances[0].InstanceId, ipAddress: *runResult.Instances[0].PrivateIpAddress}, nil
}

func getAmiIDFromSSM(ctx context.Context, client *ssm.SSM, amiName string) (*string, error) {
	getParameterInput := &ssm.GetParameterInput{
		Name:           aws.String(amiName),
		WithDecryption: aws.Bool(true),
	}

	output, err := client.GetParameterWithContext(ctx, getParameterInput)
	if err != nil {
		return nil, err
	}

	return output.Parameter.Value, nil
}

func deleteEC2Instance(ctx context.Context, client *ec2.EC2, instanceID string) error {
	terminateInstanceInput := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	}

	if _, err := client.TerminateInstancesWithContext(ctx, terminateInstanceInput); err != nil {
		return err
	}
	fmt.Println("EC2 instance terminated successfully.")
	return nil
}
