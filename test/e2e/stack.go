//go:build e2e
// +build e2e

package e2e

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/iam"
)

//go:embed cfn-templates/hybrid-cfn.yaml
var cfnTemplateBody []byte

type e2eCfnStack struct {
	clusterName         string
	stackName           string
	credentialProviders []NodeadmCredentialsProvider
	clusterArn          string
	cfn                 *cloudformation.CloudFormation
	iam                 *iam.IAM
}

type e2eCfnStackOutput struct {
	ec2InstanceProfile string
	ssmNodeRoleName    string
	ssmNodeRoleARN     string
}

func (e *e2eCfnStack) deployResourcesStack(ctx context.Context) (*e2eCfnStackOutput, error) {
	resp, err := e.cfn.DescribeStacksWithContext(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(e.stackName),
	})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() != "ValidationError" {
		return nil, fmt.Errorf("looking for hybrid nodes cfn stack: %w", err)
	}
	params := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String("clusterName"),
			ParameterValue: aws.String(e.clusterName),
		},
		{
			ParameterKey:   aws.String("clusterArn"),
			ParameterValue: aws.String(e.clusterArn),
		},
	}

	for _, credProvider := range e.credentialProviders {
		params = append(params, &cloudformation.Parameter{
			ParameterKey:   aws.String(string(credProvider.Name())),
			ParameterValue: aws.String("true"),
		})
	}

	if len(resp.Stacks) == 0 {
		logger.Info("Creating hybrid nodes stack", "stackName", e.stackName)
		_, err = e.cfn.CreateStackWithContext(ctx, &cloudformation.CreateStackInput{
			StackName:    aws.String(e.stackName),
			TemplateBody: aws.String(string(cfnTemplateBody)),
			Parameters:   params,
			Capabilities: []*string{
				aws.String("CAPABILITY_NAMED_IAM"),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("creating hybrid nodes cfn stack: %w", err)
		}

		logger.Info("Waiting for hybrid nodes stack to be created", "stackName", e.stackName)
		err = e.cfn.WaitUntilStackCreateCompleteWithContext(ctx, &cloudformation.DescribeStacksInput{
			StackName: aws.String(e.stackName),
		}, request.WithWaiterDelay(request.ConstantWaiterDelay(2*time.Second)))
		if err != nil {
			return nil, fmt.Errorf("waiting for hybrid nodes cfn stack: %w", err)
		}
	} else {
		logger.Info("Updating hybrid nodes stack", "stackName", e.stackName)
		_, err = e.cfn.UpdateStackWithContext(ctx, &cloudformation.UpdateStackInput{
			StackName: aws.String(e.stackName),
			Capabilities: []*string{
				aws.String("CAPABILITY_NAMED_IAM"),
			},
			TemplateBody: aws.String(string(cfnTemplateBody)),
			Parameters:   params,
		})

		if aerr, ok := err.(awserr.Error); err != nil && (!ok || aerr.Message() != "No updates are to be performed.") {
			return nil, fmt.Errorf("updating hybrid nodes cfn stack: %w", err)
		} else if ok && aerr.Message() == "No updates are to be performed." {
			logger.Info("No updates are to be performed for hybrid nodes stack", "stackName", e.stackName)
			// Skip waiting for update completion since no update occurred
			return e.readStackOutput(ctx)
		}

		logger.Info("Waiting for hybrid nodes stack to be updated", "stackName", e.stackName)
		err = e.cfn.WaitUntilStackUpdateCompleteWithContext(ctx, &cloudformation.DescribeStacksInput{
			StackName: aws.String(e.stackName),
		}, request.WithWaiterDelay(request.ConstantWaiterDelay(5*time.Second)))
		if err != nil {
			return nil, fmt.Errorf("waiting for hybrid nodes cfn stack: %w", err)
		}
	}

	return e.readStackOutput(ctx)
}

func (e *e2eCfnStack) readStackOutput(ctx context.Context) (*e2eCfnStackOutput, error) {
	resp, err := e.cfn.DescribeStacksWithContext(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(e.stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describing hybrid nodes cfn stack: %w", err)
	}

	result := &e2eCfnStackOutput{}
	// extract relevant stack outputs
	for _, output := range resp.Stacks[0].Outputs {
		switch aws.StringValue(output.OutputKey) {
		case "EC2InstanceProfile":
			result.ec2InstanceProfile = *output.OutputValue
		case "SSMNodeRoleName":
			result.ssmNodeRoleName = *output.OutputValue
		case "SSMNodeRoleARN":
			result.ssmNodeRoleARN = *output.OutputValue
		}
	}

	logger.Info("E2E resources stack deployed successfully", "stackName", e.stackName)
	return result, nil
}

func (e *e2eCfnStack) deleteResourceStack(ctx context.Context) error {
	_, err := e.cfn.DeleteStackWithContext(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(e.stackName),
	})
	if err != nil {
		return fmt.Errorf("deleting hybrid nodes cfn stack: %w", err)
	}
	err = e.cfn.WaitUntilStackDeleteCompleteWithContext(ctx,
		&cloudformation.DescribeStacksInput{StackName: aws.String(e.stackName)},
		request.WithWaiterDelay(request.ConstantWaiterDelay(2*time.Second)),
		request.WithWaiterMaxAttempts(240))
	if err != nil {
		return fmt.Errorf("waiting for hybrid nodes cfn stack: %w", err)
	}
	logger.Info("E2E resources stack deleted successfully", "stackName", e.stackName)
	return nil
}
