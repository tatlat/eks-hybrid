//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	ssmv2 "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/go-logr/logr"
)

func createSSMActivation(ctx context.Context, client *ssmv2.Client, iamRole string, ssmActivationName string) (*ssmv2.CreateActivationOutput, error) {
	// Define the input for the CreateActivation API
	input := &ssmv2.CreateActivationInput{
		IamRole:             aws.String(iamRole),
		RegistrationLimit:   aws.Int32(2),
		DefaultInstanceName: aws.String(ssmActivationName),
	}

	// Call CreateActivation to create the SSM activation
	result, err := client.CreateActivation(ctx, input, func(o *ssmv2.Options) {
		o.RetryMaxAttempts = 20
		o.RetryMode = awsv2.RetryModeAdaptive
	})

	if err != nil {
		return nil, fmt.Errorf("creating SSM activation: %v", err)
	}

	return result, nil
}

type ssmConfig struct {
	client     *ssm.SSM
	instanceID string
	commands   []string
}

func (s *ssmConfig) runCommandsOnInstance(ctx context.Context, logger logr.Logger) ([]ssm.GetCommandInvocationOutput, error) {
	outputs := []ssm.GetCommandInvocationOutput{}
	for _, command := range s.commands {
		logger.Info("Running command: ", "command", command)
		input := &ssm.SendCommandInput{
			DocumentName: aws.String("AWS-RunShellScript"),
			Parameters: map[string][]*string{
				"commands": aws.StringSlice([]string{command}),
			},
			InstanceIds: []*string{aws.String(s.instanceID)},
		}
		output, err := s.client.SendCommandWithContext(ctx, input)
		// Retry if the ThrottlingException occurred
		for err != nil && isThrottlingException(err) {
			logger.Info("ThrottlingException encountered, retrying..")
			output, err = s.client.SendCommandWithContext(ctx, input)
		}
		invocationInput := &ssm.GetCommandInvocationInput{
			CommandId:  output.Command.CommandId,
			InstanceId: aws.String(s.instanceID),
		}
		// Will wait on Pending, InProgress, or Cancelling until we reach a terminal status of Success, Cancelled, Failed, TimedOut
		_ = s.client.WaitUntilCommandExecutedWithContext(ctx, invocationInput)
		invocationOutput, err := s.client.GetCommandInvocationWithContext(ctx, invocationInput)
		if err != nil {
			return nil, fmt.Errorf("got an error calling GetCommandInvocation: %w", err)
		}
		logger.Info("Command output", "output", invocationOutput.String())
		outputs = append(outputs, *invocationOutput)
	}
	return outputs, nil
}

func (s *ssmConfig) runCommandsOnInstanceWaitForInProgress(ctx context.Context, logger logr.Logger) ([]ssm.GetCommandInvocationOutput, error) {
	outputs := []ssm.GetCommandInvocationOutput{}
	logger.Info(fmt.Sprintf("Running command: %v\n", s.commands))
	input := &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]*string{
			"commands": aws.StringSlice(s.commands),
		},
		InstanceIds: []*string{aws.String(s.instanceID)},
	}
	output, err := s.client.SendCommandWithContext(ctx, input)
	// Retry if the ThrottlingException occurred
	for err != nil && isThrottlingException(err) {
		logger.Info("ThrottlingException encountered, retrying..")
		output, err = s.client.SendCommandWithContext(ctx, input)
	}
	if err != nil {
		return nil, fmt.Errorf("got an error calling SendCommandWithContext: %w", err)
	}
	invocationInput := &ssm.GetCommandInvocationInput{
		CommandId:  output.Command.CommandId,
		InstanceId: aws.String(s.instanceID),
	}
	opts := func(w *request.Waiter) {
		w.Acceptors = []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathWaiterMatch, Argument: "Status",
				Expected: "InProgress",
			},
		}
	}
	_ = s.client.WaitUntilCommandExecutedWithContext(ctx, invocationInput, opts)
	invocationOutput, err := s.client.GetCommandInvocationWithContext(ctx, invocationInput)
	if err != nil {
		return nil, fmt.Errorf("got an error calling GetCommandInvocation: %w", err)
	}
	logger.Info(invocationOutput.String())
	outputs = append(outputs, *invocationOutput)

	return outputs, nil
}

func isThrottlingException(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok && awsErr != nil {
		return awsErr.Code() == "ThrottlingException"
	}
	return false
}
