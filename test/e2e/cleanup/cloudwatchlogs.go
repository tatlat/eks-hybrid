package cleanup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/go-logr/logr"
)

const (
	// 14 days is the retention period for log groups
	// on the 15th day, the log group will be deleted
	clusterLogGroupRetentionDays = 15
	clusterLogGroupPrefix        = "/aws/eks"
	clusterLogGroupSuffix        = "/cluster"
)

type CloudWatchLogsCleaner struct {
	client        *cloudwatchlogs.Client
	logger        logr.Logger
	taggingClient *ResourceTaggingClient
}

func NewCloudWatchLogsCleaner(client *cloudwatchlogs.Client, taggingClient *ResourceTaggingClient, logger logr.Logger) *CloudWatchLogsCleaner {
	return &CloudWatchLogsCleaner{
		client:        client,
		logger:        logger,
		taggingClient: taggingClient,
	}
}

// ListLogGroups lists all log groups that match the filter input
// the instance age threshold is not used for log groups, instead we use 15 days as the threshold
func (c *CloudWatchLogsCleaner) ListLogGroups(ctx context.Context, filterInput FilterInput) ([]string, error) {
	// describe-logs-groups does not return the tags and since we expect there to be a number of log groups (~400)
	// for a specific sweeper run, we use the resource tagging api to get the tags for all log groups based on the filter input
	// instead of making seperate tagging api calls for each log group
	groups, err := c.allGroupsWithTags(ctx, filterInput)
	if err != nil {
		return nil, fmt.Errorf("listing log groups: %w", err)
	}

	groupNamePattern := fmt.Sprintf("%s/", clusterLogGroupPrefix)
	if filterInput.ClusterName != "" {
		groupNamePattern = groupNamePattern + filterInput.ClusterName
	} else if filterInput.ClusterNamePrefix != "" {
		groupNamePattern = groupNamePattern + filterInput.ClusterNamePrefix
	}
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(c.client, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(groupNamePattern),
	})

	var logGroupNames []string
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing log groups: %w", err)
		}

		for _, logGroup := range output.LogGroups {
			tags, ok := groups[*logGroup.LogGroupName]
			if ok && shouldDeleteLogGroup(logGroup, tags, filterInput) {
				logGroupNames = append(logGroupNames, *logGroup.LogGroupName)
			}
		}
	}

	return logGroupNames, nil
}

// ex: arn:aws:logs:us-west-2:<account-id>:log-group:/aws/eks/nodeadm-e2e-tests-74c4e460-3af4-4a6e-abcd-62713cba1c52/cluster
func parseLogGroupNameFromARN(arn string) string {
	lastSlashIndex := strings.Index(arn, "/")
	return arn[lastSlashIndex:]
}

func (c *CloudWatchLogsCleaner) allGroupsWithTags(ctx context.Context, filterInput FilterInput) (map[string][]Tag, error) {
	resourceARNs, err := c.taggingClient.GetResourcesWithClusterTag(ctx, "logs:log-group", filterInput)
	if err != nil {
		return nil, fmt.Errorf("listing roles anywhere profiles: %w", err)
	}

	groups := map[string][]Tag{}
	for resourceARN, tags := range resourceARNs {
		groups[parseLogGroupNameFromARN(resourceARN)] = tags
	}
	return groups, nil
}

func (c *CloudWatchLogsCleaner) DeleteLogGroup(ctx context.Context, logGroupName string) error {
	_, err := c.client.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: &logGroupName,
	})
	if err != nil {
		return fmt.Errorf("deleting log group %s: %w", logGroupName, err)
	}
	c.logger.Info("Deleted log group", "logGroupName", logGroupName)
	return nil
}

func shouldDeleteLogGroup(logGroup types.LogGroup, tags []Tag, filterInput FilterInput) bool {
	filterInput.InstanceAgeThreshold = clusterLogGroupRetentionDays * 24 * time.Hour
	return shouldDeleteResource(ResourceWithTags{
		ID:           *logGroup.LogGroupName,
		CreationTime: time.UnixMilli(aws.ToInt64(logGroup.CreationTime)),
		Tags:         tags,
	}, filterInput)
}
