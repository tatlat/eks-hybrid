package ssm

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsSsm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func isInstanceManaged(ctx context.Context, client SSMClient, instanceId string) (bool, error) {
	output, err := client.DescribeInstanceInformation(ctx, &awsSsm.DescribeInstanceInformationInput{
		Filters: []types.InstanceInformationStringFilter{
			{
				Key:    aws.String("InstanceIds"),
				Values: []string{instanceId},
			},
		},
	})
	if err != nil {
		return false, err
	}

	return len(output.InstanceInformationList) > 0, nil
}

func deregister(ctx context.Context, client SSMClient, instanceId string) error {
	_, err := client.DeregisterManagedInstance(ctx, &awsSsm.DeregisterManagedInstanceInput{
		InstanceId: &instanceId,
	})
	return err
}
