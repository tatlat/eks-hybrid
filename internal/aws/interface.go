package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type Config interface {
	ConfigureAws(ctx context.Context) error
	GetConfig() *aws.Config
}
