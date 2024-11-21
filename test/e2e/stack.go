//go:build e2e
// +build e2e

package e2e

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/go-logr/logr"
)

//go:embed cfn-templates/hybrid-cfn.yaml
var cfnTemplateBody []byte

type e2eCfnStack struct {
	clusterName            string
	stackName              string
	credentialProviders    []NodeadmCredentialsProvider
	clusterArn             string
	cfn                    *cloudformation.CloudFormation
	iam                    *iam.IAM
	iamRolesAnywhereCACert []byte
}

type e2eCfnStackOutput struct {
	EC2Role            string `json:"EC2Role"`
	InstanceProfileARN string `json:"instanceProfileARN"`
	SSMNodeRoleName    string `json:"ssmNodeRoleName"`
	SSMNodeRoleARN     string `json:"ssmNodeRoleARN"`
	IRANodeRoleName    string `json:"iraNodeRoleName"`
	IRANodeRoleARN     string `json:"iraNodeRoleARN"`
	IRATrustAnchorARN  string `json:"iraTrustAnchorARN"`
	IRAProfileARN      string `json:"iraProfileARN"`
}

func (e *e2eCfnStack) deploy(ctx context.Context, logger logr.Logger) (*e2eCfnStackOutput, error) {
	if err := e.deployStack(ctx, logger); err != nil {
		return nil, err
	}

	output, err := e.readStackOutput(ctx, logger)
	if err != nil {
		return nil, err
	}

	// We create the instance profile manually instead of as part of the CFN stack because it's faster.
	// This sucks because of the complexity it adds both to create and delete, having to deal with
	// partial creations where the instance profile might exist already and role might have been added or not.
	// But it speeds up the test about 2.5 minutes, so it's worth it. For some reason, creating
	// instance profiles from CFN
	// is very slow: https://repost.aws/questions/QUoU5UybeUR2S2iYNEJiStiQ/cloudformation-provisioning-of-aws-iam-instanceprofile-takes-a-long-time
	// I suspect this is because CFN has a hardcoded ~2.5 minutes wait after instance profile creation before
	// considering it "created". Probably to avoid eventual consistency issues when using the instance profile in
	// another resource immediately after. We have to deal with that problem ourselves now by retrying the ec2 instance
	// creation on "invalid IAM instance profile" error.
	output.InstanceProfileARN, err = e.createInstanceProfile(ctx, logger, output.EC2Role)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (e *e2eCfnStack) deployStack(ctx context.Context, logger logr.Logger) error {
	resp, err := e.cfn.DescribeStacksWithContext(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(e.stackName),
	})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() != "ValidationError" {
		return fmt.Errorf("looking for hybrid nodes cfn stack: %w", err)
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
		{
			ParameterKey:   aws.String("caBundleCert"),
			ParameterValue: aws.String(string(e.iamRolesAnywhereCACert)),
		},
	}

	for _, credProvider := range e.credentialProviders {
		params = append(params, &cloudformation.Parameter{
			// assume that the name of the param is the same as the name of the provider minus the dashes
			ParameterKey:   aws.String(strings.ReplaceAll(string(credProvider.Name()), "-", "")),
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
			return fmt.Errorf("creating hybrid nodes cfn stack: %w", err)
		}

		logger.Info("Waiting for hybrid nodes stack to be created", "stackName", e.stackName)
		err = e.cfn.WaitUntilStackCreateCompleteWithContext(ctx, &cloudformation.DescribeStacksInput{
			StackName: aws.String(e.stackName),
		}, request.WithWaiterDelay(request.ConstantWaiterDelay(2*time.Second)))
		if err != nil {
			return fmt.Errorf("waiting for hybrid nodes cfn stack: %w", err)
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
			return fmt.Errorf("updating hybrid nodes cfn stack: %w", err)
		} else if ok && aerr.Message() == "No updates are to be performed." {
			logger.Info("No updates are to be performed for hybrid nodes stack", "stackName", e.stackName)
			// Skip waiting for update completion since no update occurred
			return nil
		}

		logger.Info("Waiting for hybrid nodes stack to be updated", "stackName", e.stackName)
		err = e.cfn.WaitUntilStackUpdateCompleteWithContext(ctx, &cloudformation.DescribeStacksInput{
			StackName: aws.String(e.stackName),
		}, request.WithWaiterDelay(request.ConstantWaiterDelay(5*time.Second)))
		if err != nil {
			return fmt.Errorf("waiting for hybrid nodes cfn stack: %w", err)
		}
	}

	return nil
}

func (s *e2eCfnStack) createInstanceProfile(ctx context.Context, logger logr.Logger, roleName string) (instanceProfileArn string, err error) {
	instanceProfileName := s.instanceProfileName(roleName)

	instanceProfile, err := s.iam.GetInstanceProfileWithContext(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
	})
	var instanceProfileHasRole bool
	if isNotFound(err) {
		logger.Info("Creating instance profile", "instanceProfileName", instanceProfileName)
		instanceProfileArnOut, err := s.iam.CreateInstanceProfileWithContext(ctx, &iam.CreateInstanceProfileInput{
			InstanceProfileName: aws.String(instanceProfileName),
			Path:                aws.String("/"),
		})
		if err != nil {
			return "", err
		}
		instanceProfileArn = *instanceProfileArnOut.InstanceProfile.Arn
		instanceProfileHasRole = false
	} else if err != nil {
		return "", err
	} else {
		logger.Info("Instance profile already exists", "instanceProfileName", instanceProfileName)
		instanceProfileArn = *instanceProfile.InstanceProfile.Arn
		if len(instanceProfile.InstanceProfile.Roles) > 0 {
			instanceProfileHasRole = true
		} else {
			instanceProfileHasRole = false
		}
	}

	if instanceProfileHasRole {
		logger.Info("Instance profile already has a role", "instanceProfileName", instanceProfileName)
	} else {
		logger.Info("Adding role to instance profile", "roleName", roleName)
		_, err = s.iam.AddRoleToInstanceProfileWithContext(ctx, &iam.AddRoleToInstanceProfileInput{
			InstanceProfileName: aws.String(instanceProfileName),
			RoleName:            aws.String(roleName),
		})
		if err != nil {
			return "", err
		}

	}

	return instanceProfileArn, nil
}

func (s *e2eCfnStack) instanceProfileName(roleName string) string {
	return roleName
}

func (e *e2eCfnStack) readStackOutput(ctx context.Context, logger logr.Logger) (*e2eCfnStackOutput, error) {
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
		case "EC2Role":
			result.EC2Role = *output.OutputValue
		case "SSMNodeRoleName":
			result.SSMNodeRoleName = *output.OutputValue
		case "SSMNodeRoleARN":
			result.SSMNodeRoleARN = *output.OutputValue
		case "IRANodeRoleName":
			result.IRANodeRoleName = *output.OutputValue
		case "IRANodeRoleARN":
			result.IRANodeRoleARN = *output.OutputValue
		case "IRATrustAnchorARN":
			result.IRATrustAnchorARN = *output.OutputValue
		case "IRAProfileARN":
			result.IRAProfileARN = *output.OutputValue
		}
	}

	logger.Info("E2E resources stack deployed successfully", "stackName", e.stackName)
	return result, nil
}

func (e *e2eCfnStack) delete(ctx context.Context, logger logr.Logger, output *e2eCfnStackOutput) error {
	instanceProfileName := e.instanceProfileName(output.EC2Role)
	logger.Info("Deleting instance profile", "instanceProfileName", instanceProfileName)
	instanceProfile, err := e.iam.GetInstanceProfileWithContext(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
	})
	if err != nil {
		return err
	}
	if _, err := e.iam.RemoveRoleFromInstanceProfileWithContext(ctx, &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		RoleName:            instanceProfile.InstanceProfile.Roles[0].RoleName,
	}); err != nil {
		return err
	}
	if _, err := e.iam.DeleteInstanceProfileWithContext(ctx, &iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
	}); err != nil {
		return fmt.Errorf("deleting instance profile: %w", err)
	}

	_, err = e.cfn.DeleteStackWithContext(ctx, &cloudformation.DeleteStackInput{
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

func isNotFound(err error) bool {
	aerr, ok := err.(awserr.Error)
	return err != nil && ok && aerr.Code() == "NoSuchEntity"
}
