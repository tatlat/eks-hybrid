//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/smithy-go"
)

// instanceConfig holds the configuration for the EC2 instance.
type ec2InstanceConfig struct {
	instanceName       string
	amiID              string
	instanceType       string
	instanceProfileARN string
	volumeSize         int32
	userData           []byte
	subnetID           string
	securityGroupID    string
}

type ec2Instance struct {
	instanceID string
	ipAddress  string
}

func (e *ec2InstanceConfig) create(ctx context.Context, ec2Client *ec2.Client, ssmClient *ssm.SSM) (ec2Instance, error) {
	instances, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{e.instanceName},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running", "pending"},
			},
		},
	})
	if err != nil {
		return ec2Instance{}, fmt.Errorf("describing EC2 instances: %w", err)
	}

	if len(instances.Reservations) > 1 {
		return ec2Instance{}, fmt.Errorf("more than one reservation for instances with the name %s found", e.instanceName)
	}

	if len(instances.Reservations) == 1 && len(instances.Reservations[0].Instances) > 1 {
		return ec2Instance{}, fmt.Errorf("more than one instance with the name %s found", e.instanceName)
	}

	if len(instances.Reservations) == 1 && len(instances.Reservations[0].Instances) == 1 {
		return ec2Instance{
			instanceID: *instances.Reservations[0].Instances[0].InstanceId,
			ipAddress:  *instances.Reservations[0].Instances[0].PrivateIpAddress,
		}, nil
	}

	userDataEncoded := base64.StdEncoding.EncodeToString(e.userData)

	runResult, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String(e.amiID),
		InstanceType: types.InstanceType(e.instanceType),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		IamInstanceProfile: &types.IamInstanceProfileSpecification{
			Arn: aws.String(e.instanceProfileARN),
		},
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &types.EbsBlockDevice{
					VolumeSize: aws.Int32(e.volumeSize),
				},
			},
		},
		NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex: aws.Int32(0),
				SubnetId:    aws.String(e.subnetID),
				Groups:      []string{e.securityGroupID},
			},
		},
		UserData: aws.String(userDataEncoded),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(e.instanceName),
					},
				},
			},
		},
		MetadataOptions: &types.InstanceMetadataOptionsRequest{
			HttpTokens:   types.HttpTokensStateRequired,
			HttpEndpoint: types.InstanceMetadataEndpointStateEnabled,
		},
	}, func(o *ec2.Options) {
		o.Retryer = newDefaultRunEC2Retrier()
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

func deleteEC2Instance(ctx context.Context, client *ec2.Client, instanceID string) error {
	terminateInstanceInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}

	if _, err := client.TerminateInstances(ctx, terminateInstanceInput); err != nil {
		return err
	}
	return nil
}

func newDefaultRunEC2Retrier() runInstanceRetrier {
	return runInstanceRetrier{
		Standard:   retry.NewStandard(),
		maxRetries: 60,
		backoff:    2 * time.Second,
	}
}

type runInstanceRetrier struct {
	*retry.Standard
	maxRetries int
	backoff    time.Duration
}

func (c runInstanceRetrier) IsErrorRetryable(err error) bool {
	if c.Standard.IsErrorRetryable(err) {
		return true
	}

	var awsErr smithy.APIError
	if ok := errors.As(err, &awsErr); ok {
		// We retry invalid instance profile errors because sometimes there is a delay between creating
		// the instance profile and that profile being available in EC2. We trust that if this error comes
		// back, it's just an eventual consistency issue and not that our setup code is not creating the
		// instance profile correctly.
		// The error message can be:
		// - Invalid IAM Instance Profile name
		// - Invalid IAM Instance Profile ARN
		// Depending if the input uses the name or the ARN in the params.
		if awsErr.ErrorCode() == "InvalidParameterValue" && strings.Contains(awsErr.ErrorMessage(), "Invalid IAM Instance Profile") {
			return true
		}
	}

	return false
}

func (c runInstanceRetrier) MaxAttempts() int {
	return c.maxRetries
}

func (c runInstanceRetrier) RetryDelay(attempt int, err error) (time.Duration, error) {
	return c.backoff, nil
}

func rebootEC2Instance(ctx context.Context, client *ec2.Client, instanceID string) error {
	rebootInstanceInput := &ec2.RebootInstancesInput{
		InstanceIds: []string{instanceID},
	}

	if _, err := client.RebootInstances(ctx, rebootInstanceInput); err != nil {
		return err
	}
	return nil
}
