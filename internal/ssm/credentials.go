package ssm

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/util/file"
)

const awsSharedCredentialsFileEnvVar = "AWS_SHARED_CREDENTIALS_FILE"

func WaitForAWSConfig(ctx context.Context, nodeConfig *api.NodeConfig, backoff time.Duration) (aws.Config, error) {
	credsFile := awsCredsFile()
	for !file.Exists(credsFile) {
		select {
		case <-ctx.Done():
			return aws.Config{}, fmt.Errorf("ssm AWS creds file %s hasn't been created on time: %w", credsFile, ctx.Err())
		case <-time.After(backoff):
		}
	}

	return config.LoadDefaultConfig(ctx,
		config.WithRegion(nodeConfig.Spec.Cluster.Region),
		config.WithSharedCredentialsFiles([]string{credsFile}),
		// important to pass empty slice instead of nil to stop
		// the SDK from using the default paths
		config.WithSharedConfigFiles([]string{}),
		// This is helpful if the machine happens to be running on an EC2 instance
		// so we avoid defaulting to IMDS by mistake.
		config.WithEC2IMDSClientEnableState(imds.ClientDisabled),
	)
}

func awsCredsFile() string {
	credsFile := awsCredentialsFilePath
	if cFile, ok := os.LookupEnv(awsSharedCredentialsFileEnvVar); ok {
		credsFile = cFile
	}
	return credsFile
}
