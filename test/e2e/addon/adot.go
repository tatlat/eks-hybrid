package addon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"

	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
)

const (
	adotAddonName = "adot"
	adotNamespace = "opentelemetry-operator-system"

	adotOperatorDeployment = "opentelemetry-operator"

	// adotCollectorDaemonSet is the DaemonSet created by the addon when the
	// containerLogs pipeline is enabled via configuration values.
	// The OpenTelemetry operator appends "-collector" to the collector name.
	adotCollectorDaemonSet = "adot-col-container-logs-collector"

	// adotCollectorServiceAcct is the service account used by the container logs
	// collector pods. This must match the PodIdentityAssociation service account.
	adotCollectorServiceAcct = "adot-col-container-logs"

	adotWaitTimeout          = 5 * time.Minute
	adotLogGroupPollInterval = 15 * time.Second
	adotLogGroupPollTimeout  = 5 * time.Minute
)

// ADOTTest tests the AWS Distro for OpenTelemetry (ADOT) addon.
// ADOT requires cert-manager to be installed first.
// Validation uses the Container Logs pipeline to ship pod logs to CloudWatch Logs,
// then verifies that log entries appear in the expected log group.
//
// The ADOT EKS addon creates the collector DaemonSet automatically when the
// containerLogs pipeline is enabled in the addon's configuration values.
// Pod Identity is used to grant the collector service account CloudWatch Logs permissions.
type ADOTTest struct {
	Cluster            string
	K8S                clientgo.Interface
	EKSClient          *eks.Client
	CloudWatchClient   *cloudwatchlogs.Client
	PodIdentityRoleArn string
	Logger             logr.Logger
	addon              *Addon
	available          bool
}

// logGroupName returns the CloudWatch log group that the container logs collector writes to.
func (a *ADOTTest) logGroupName() string {
	return fmt.Sprintf("%s/container/logs", a.Cluster)
}

// adotConfiguration returns the JSON addon configuration values that enable the
// containerLogs pipeline and configure the CloudWatch exporter.
func (a *ADOTTest) adotConfiguration() (string, error) {
	cfg := map[string]interface{}{
		"collector": map[string]interface{}{
			"containerLogs": map[string]interface{}{
				"pipelines": map[string]interface{}{
					"logs": map[string]interface{}{
						"cloudwatchLogs": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"exporters": map[string]interface{}{
					"awscloudwatchlogs": map[string]interface{}{
						"log_group_name": a.logGroupName(),
					},
				},
			},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshaling ADOT configuration: %w", err)
	}
	return string(b), nil
}

// Setup installs the ADOT EKS addon with the containerLogs pipeline enabled and waits
// for both the operator deployment and the collector DaemonSet to be ready.
// The addon creates the DaemonSet automatically from the configuration values.
// Pod Identity grants the collector service account CloudWatch Logs write permissions.
// If the addon is not available in the current region/partition, Setup returns nil
// and marks the test as unavailable so Validate becomes a no-op.
func (a *ADOTTest) Setup(ctx context.Context) error {
	a.Logger.Info("Setting up ADOT test")

	cfg, err := a.adotConfiguration()
	if err != nil {
		return err
	}

	a.addon = &Addon{
		Cluster:       a.Cluster,
		Namespace:     adotNamespace,
		Name:          adotAddonName,
		Configuration: cfg,
		PodIdentityAssociations: []PodIdentityAssociation{
			{
				RoleArn:        a.PodIdentityRoleArn,
				ServiceAccount: adotCollectorServiceAcct,
			},
		},
	}

	if err := a.addon.CreateAndWaitForActive(ctx, a.EKSClient, a.K8S, a.Logger); err != nil {
		if errors.Is(err, ErrAddonNotAvailable) {
			a.Logger.Info("ADOT addon is not available in this region, skipping ADOT validation")
			a.available = false
			return nil
		}
		return fmt.Errorf("failed to create ADOT addon: %w", err)
	}

	// Wait for the OpenTelemetry operator deployment to be ready.
	if err := kubernetes.DeploymentWaitForReplicas(ctx, adotWaitTimeout, a.K8S, adotNamespace, adotOperatorDeployment); err != nil {
		return fmt.Errorf("waiting for ADOT operator deployment to be ready: %w", err)
	}

	// Wait for the container logs collector DaemonSet, which the addon creates
	// automatically when containerLogs is enabled in the configuration values.
	if err := kubernetes.DaemonSetWaitForReady(ctx, a.Logger, a.K8S, adotNamespace, adotCollectorDaemonSet); err != nil {
		return fmt.Errorf("waiting for ADOT container logs DaemonSet to be ready: %w", err)
	}

	a.available = true
	a.Logger.Info("ADOT setup complete")
	return nil
}

// Validate checks that the ADOT container logs collector is shipping logs to CloudWatch.
// It polls until the expected log group exists and contains at least one log stream.
// If the addon was unavailable, this is a no-op.
func (a *ADOTTest) Validate(ctx context.Context) error {
	if !a.available {
		a.Logger.Info("ADOT not available in this region, skipping ADOT validation")
		return nil
	}

	a.Logger.Info("Validating ADOT container logs", "logGroup", a.logGroupName())

	err := wait.PollUntilContextTimeout(ctx, adotLogGroupPollInterval, adotLogGroupPollTimeout, true, func(ctx context.Context) (bool, error) {
		resp, err := a.CloudWatchClient.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String(a.logGroupName()),
		})
		if err != nil {
			a.Logger.Error(err, "Error describing CloudWatch log groups, will retry")
			return false, nil
		}

		for _, lg := range resp.LogGroups {
			if aws.ToString(lg.LogGroupName) == a.logGroupName() {
				streams, err := a.CloudWatchClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String(a.logGroupName()),
				})
				if err != nil {
					a.Logger.Error(err, "Error describing log streams, will retry")
					return false, nil
				}
				if len(streams.LogStreams) > 0 {
					a.Logger.Info("ADOT container logs validation successful",
						"logGroup", a.logGroupName(),
						"streamCount", len(streams.LogStreams))
					return true, nil
				}
				a.Logger.Info("Log group exists but no streams yet, waiting", "logGroup", a.logGroupName())
				return false, nil
			}
		}

		a.Logger.Info("Waiting for CloudWatch log group to be created", "logGroup", a.logGroupName())
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("ADOT container logs did not appear in CloudWatch log group %q within %v: %w",
			a.logGroupName(), adotLogGroupPollTimeout, err)
	}

	return nil
}

// PrintLogs collects logs from the ADOT operator for debugging.
func (a *ADOTTest) PrintLogs(ctx context.Context) error {
	if !a.available {
		return nil
	}

	logs, err := kubernetes.FetchLogs(ctx, a.K8S, adotOperatorDeployment, adotNamespace)
	if err != nil {
		return fmt.Errorf("failed to collect ADOT operator logs: %w", err)
	}

	a.Logger.Info("Logs for ADOT operator", "logs", logs)
	return nil
}

// Cleanup removes the ADOT addon and the CloudWatch log group created during the test.
func (a *ADOTTest) Cleanup(ctx context.Context) error {
	if a.addon == nil {
		return nil
	}

	a.Logger.Info("Cleaning up ADOT resources")
	if err := a.addon.Delete(ctx, a.EKSClient, a.Logger); err != nil {
		a.Logger.Error(err, "Failed to delete ADOT addon")
	}

	a.Logger.Info("Deleting CloudWatch log group", "logGroup", a.logGroupName())
	_, err := a.CloudWatchClient.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(a.logGroupName()),
	})
	if err != nil {
		if strings.Contains(err.Error(), "ResourceNotFoundException") {
			a.Logger.Info("CloudWatch log group does not exist, nothing to delete", "logGroup", a.logGroupName())
		} else {
			a.Logger.Error(err, "Failed to delete CloudWatch log group", "logGroup", a.logGroupName())
		}
	} else {
		a.Logger.Info("Successfully deleted CloudWatch log group", "logGroup", a.logGroupName())
	}

	return nil
}
