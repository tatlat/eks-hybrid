package ssm

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/go-logr/logr"
)

func RunNodeadmUninstall(ctx context.Context, client *ssm.SSM, instanceID string, logger logr.Logger) error {
	commands := []string{
		// TODO: @pjshah run uninstall without node-validation and pod-validation flags after adding cordon and drain node functionality
		"set -eux",
		"trap \"/tmp/log-collector.sh 'post-uninstall' 'post-final-uninstall'\" EXIT",
		"sudo /tmp/nodeadm uninstall -skip node-validation,pod-validation",
		"sudo cloud-init clean --logs",
		"sudo rm -rf /var/lib/cloud/instances",
	}
	ssmConfig := &ssmConfig{
		client:     client,
		instanceID: instanceID,
		commands:   commands,
	}
	// TODO: handle provider specific ssm command wait status
	outputs, err := ssmConfig.runCommandsOnInstanceWaitForInProgress(ctx, logger)
	if err != nil {
		return fmt.Errorf("running SSM command: %w", err)
	}
	logger.Info("Nodeadm Uninstall", "output", outputs)
	for _, output := range outputs {
		if *output.Status != "Success" && *output.Status != "InProgress" {
			return fmt.Errorf("node uninstall SSM command did not properly reach InProgress")
		}
	}
	return nil
}
