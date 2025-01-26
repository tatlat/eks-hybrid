package ssm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/go-logr/logr"

	e2eCommands "github.com/aws/eks-hybrid/test/e2e/commands"
)

const (
	commandExecTimeout = 10 * time.Minute
	commandWaitTimeout = commandExecTimeout + time.Minute
)

// ssm commands run as root user on jumpbox
func makeSshCommand(instanceIP string, commands []string) string {
	return fmt.Sprintf("ssh %s \"%s\"", instanceIP, strings.Replace(strings.Join(commands, ";"), "\"", "\\\"", -1))
}

type SSHOnSSM struct {
	client            *ssm.Client
	jumpboxInstanceId string
	logger            logr.Logger
}

func NewSSHOnSSMCommandRunner(client *ssm.Client, jumpboxInstanceId string, logger logr.Logger) e2eCommands.RemoteCommandRunner {
	return &SSHOnSSM{
		client:            client,
		jumpboxInstanceId: jumpboxInstanceId,
		logger:            logger,
	}
}

func (s *SSHOnSSM) Run(ctx context.Context, ip string, commands []string) (e2eCommands.RemoteCommandOutput, error) {
	command := makeSshCommand(ip, commands)
	return RunCommand(ctx, s.client, s.jumpboxInstanceId, command, s.logger)
}

func RunCommand(ctx context.Context, client *ssm.Client, instanceId, command string, logger logr.Logger) (e2eCommands.RemoteCommandOutput, error) {
	logger.Info(fmt.Sprintf("Running command: %v\n", command))
	input := &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]string{
			"commands":         {command},
			"executionTimeout": {fmt.Sprintf("%g", commandExecTimeout.Seconds())},
		},
		InstanceIds: []string{instanceId},
	}
	optsFn := func(opts *ssm.Options) {
		opts.RetryMode = aws.RetryModeAdaptive
		opts.RetryMaxAttempts = 60
	}
	output, err := client.SendCommand(ctx, input, optsFn)
	if err != nil {
		return e2eCommands.RemoteCommandOutput{}, fmt.Errorf("got an error calling SendCommand: %w", err)
	}
	invocationInput := &ssm.GetCommandInvocationInput{
		CommandId:  output.Command.CommandId,
		InstanceId: aws.String(instanceId),
	}
	waiter := ssm.NewCommandExecutedWaiter(client)
	// Will wait on Pending, InProgress, or Cancelling until we reach a terminal status of Success, Cancelled, Failed, TimedOut
	_ = waiter.Wait(ctx, invocationInput, commandWaitTimeout)
	invocationOutput, err := client.GetCommandInvocation(ctx, invocationInput, optsFn)
	if err != nil {
		return e2eCommands.RemoteCommandOutput{}, fmt.Errorf("got an error calling GetCommandInvocation: %w", err)
	}

	commandOutput := e2eCommands.RemoteCommandOutput{
		ResponseCode:   invocationOutput.ResponseCode,
		StandardError:  *invocationOutput.StandardErrorContent,
		StandardOutput: *invocationOutput.StandardOutputContent,
		Status:         string(invocationOutput.Status),
	}

	logger.Info(fmt.Sprintf("Status: %s", commandOutput.Status))
	logger.Info(fmt.Sprintf("ResponseCode: %d", commandOutput.ResponseCode))
	logger.Info(fmt.Sprintf("Stdout: %s", commandOutput.StandardOutput))
	logger.Info(fmt.Sprintf("Stderr: %s", commandOutput.StandardError))

	return commandOutput, nil
}
