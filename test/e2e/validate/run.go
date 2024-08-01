package validate

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type TestRunner struct {
	InstanceID string
	svcSSM     *ssm.SSM
	svcEC2     *ec2.EC2
	commands   []string
}

func NewTestRunner() *TestRunner {
	return &TestRunner{"", nil, nil, []string{}}
}

func (tr *TestRunner) RegisterCommands(command ...string) {
	tr.commands = append(tr.commands, command...)
}

func (tr *TestRunner) Run() []error {
	var errs []error
	for _, v := range tr.commands {
		err := tr.RunCommand(v)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (tr *TestRunner) RunCommand(command string) error {
	instanceID := tr.InstanceID
	svc := tr.svcSSM

	// Define the SSM command input
	sendCommandInput := &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"), // AWS-RunShellScript is a pre-defined SSM document
		InstanceIds:  aws.StringSlice([]string{instanceID}),
		Parameters: map[string][]*string{
			"commands": {
				aws.String(command), // Commands to run on the instances
			},
		},
	}

	// Send the SSM command
	output, err := svc.SendCommand(sendCommandInput)
	if err != nil {
		fmt.Println("Failed to send SSM command:", err)
		return err
	}

	// Print the command ID
	commandID := *output.Command.CommandId

	// Get the command status and output
	getCommandInput := &ssm.GetCommandInvocationInput{
		CommandId:  aws.String(commandID),
		InstanceId: aws.String(instanceID),
	}

	var cmdOutputStatus string
	err = TestRetrier.Retry(func() error {
		getCommandOutput, err := svc.GetCommandInvocation(getCommandInput)
		if err != nil {
			fmt.Println("Failed to get SSM command status:", err)
			return err
		}

		if *getCommandOutput.Status != "Success" && *getCommandOutput.Status != "Failed" {
			fmt.Printf("Command output: %v\n", *getCommandOutput.StandardOutputContent)
			return fmt.Errorf("command no success: %v", command)
		}

		// Print the command status and output
		fmt.Println("Command output: ", *getCommandOutput.StandardOutputContent)
		fmt.Println("Command error: ", *getCommandOutput.StandardErrorContent)
		fmt.Println("Command status: ", *getCommandOutput.Status)
		cmdOutputStatus = *getCommandOutput.Status
		return nil
	})
	if err != nil {
		fmt.Println("Failed waiting to get SSM command status:", err)
		return err
	}
	if cmdOutputStatus == "Failed" {
		return fmt.Errorf("command failed: %v", command)
	}
	return nil
}

func (tr *TestRunner) DeleteInstance() error {

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			aws.String(tr.InstanceID),
		},
	}

	svc := tr.svcEC2
	result, err := svc.TerminateInstances(input)
	if err != nil {
		return err
	}

	fmt.Println("Terminating Instance: ", *result.TerminatingInstances[0].InstanceId)
	return nil
}
