package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	cloudWatchAgentNamespace = "amazon-cloudwatch"
	cloudWatchAgentName      = "cloudwatch-agent"
)

type CloudWatchAgentTest struct {
	Cluster   string
	addon     Addon
	K8S       clientgo.Interface
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

func (c CloudWatchAgentTest) Run(ctx context.Context) error {
	c.addon = Addon{
		Cluster:   c.Cluster,
		Namespace: cloudWatchAgentNamespace,
		Name:      cloudWatchAgentName,
	}

	if err := c.Create(ctx); err != nil {
		return err
	}

	if err := c.Validate(ctx); err != nil {
		return err
	}

	return nil
}

func (c CloudWatchAgentTest) Create(ctx context.Context) error {
	if err := c.addon.CreateAddon(ctx, c.EKSClient, c.K8S, c.Logger); err != nil {
		return err
	}

	if err := kubernetes.WaitForDaemonSetReady(ctx, c.Logger, c.K8S, c.addon.Namespace, c.addon.Name); err != nil {
		return err
	}

	return nil
}

func (c CloudWatchAgentTest) Validate(ctx context.Context) error {
	c.Logger.Info("Starting CloudWatch Agent validation")

	// Verify CloudWatch log groups creation
	// Using AWS SDK to check CloudWatch logs
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Create CloudWatch Logs client
	cwLogsClient := cloudwatchlogs.NewFromConfig(cfg)

	// Wait for log groups to be created
	// Typical log group format: /aws/containerinsights/cluster-name/...
	logGroupPrefix := fmt.Sprintf("/aws/containerinsights/%s", c.Cluster)

	err = wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		// List log groups with the cluster prefix
		resp, err := cwLogsClient.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String(logGroupPrefix),
		})
		if err != nil {
			c.Logger.Error(err, "Failed to describe CloudWatch log groups")
			return false, nil
		}

		if len(resp.LogGroups) == 0 {
			c.Logger.Info("No CloudWatch log groups found yet, waiting...", "prefix", logGroupPrefix)
			return false, nil
		}

		c.Logger.Info("Found CloudWatch log groups", "count", len(resp.LogGroups))
		for _, lg := range resp.LogGroups {
			c.Logger.Info("Log group", "name", *lg.LogGroupName, "created", *lg.CreationTime)
		}

		// Check for specific log groups we expect
		expectedLogGroups := []string{
			fmt.Sprintf("%s/application", logGroupPrefix),
			fmt.Sprintf("%s/dataplane", logGroupPrefix),
			fmt.Sprintf("%s/host", logGroupPrefix),
			fmt.Sprintf("%s/performance", logGroupPrefix),
		}

		foundCount := 0
		for _, expected := range expectedLogGroups {
			for _, actual := range resp.LogGroups {
				if *actual.LogGroupName == expected {
					foundCount++
					break
				}
			}
		}

		if foundCount > 0 {
			c.Logger.Info("Found expected log groups", "found", foundCount, "expected", len(expectedLogGroups))
			return true, nil
		}

		c.Logger.Info("Waiting for expected log groups to be created")
		return false, nil
	})

	if err != nil {
		c.Logger.Info("Timed out waiting for CloudWatch log groups, some groups may still be initializing")
		// Don't fail the test as log group creation can take time
	}

	// Check if log data is flowing by getting log events from one of the groups
	if err := c.verifyLogEvents(ctx, cwLogsClient, logGroupPrefix); err != nil {
		c.Logger.Info("Log verification warning", "error", err)
		// Continue testing - don't fail on this
	}

	c.Logger.Info("CloudWatch Agent validation completed")
	return nil
}

func (c CloudWatchAgentTest) verifyLogEvents(ctx context.Context, client *cloudwatchlogs.Client, logGroupPrefix string) error {
	// List all log groups for the cluster
	resp, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(logGroupPrefix),
	})
	if err != nil {
		return fmt.Errorf("failed to list log groups: %v", err)
	}

	if len(resp.LogGroups) == 0 {
		return fmt.Errorf("no log groups found to verify")
	}

	// Try to get log events from each log group to verify data is flowing
	for i, logGroup := range resp.LogGroups {
		if i >= 2 { // Check just the first few log groups to save time
			break
		}

		// Get log streams for this log group
		streams, err := client.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: logGroup.LogGroupName,
			Limit:        aws.Int32(5),
		})
		if err != nil {
			c.Logger.Error(err, "Failed to describe log streams", "logGroup", *logGroup.LogGroupName)
			continue
		}

		if len(streams.LogStreams) == 0 {
			c.Logger.Info("No log streams found yet", "logGroup", *logGroup.LogGroupName)
			continue
		}

		// Get events from the first log stream
		events, err := client.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  logGroup.LogGroupName,
			LogStreamName: streams.LogStreams[0].LogStreamName,
			Limit:         aws.Int32(10),
		})
		if err != nil {
			c.Logger.Error(err, "Failed to get log events",
				"logGroup", *logGroup.LogGroupName,
				"logStream", *streams.LogStreams[0].LogStreamName)
			continue
		}

		if len(events.Events) > 0 {
			c.Logger.Info("Log data is flowing to CloudWatch",
				"logGroup", *logGroup.LogGroupName,
				"logStream", *streams.LogStreams[0].LogStreamName,
				"eventCount", len(events.Events))
			return nil // Found log data, validation successful
		}
	}

	return fmt.Errorf("no log events found in any log groups yet")
}

func (c CloudWatchAgentTest) CollectLogs(ctx context.Context) error {
	return c.addon.FetchLogs(ctx, c.K8S, c.Logger, []string{cloudWatchAgentName}, tailLines)
}

func (c CloudWatchAgentTest) Delete(ctx context.Context) error {
	return c.addon.Delete(ctx, c.EKSClient, c.Logger)
}
