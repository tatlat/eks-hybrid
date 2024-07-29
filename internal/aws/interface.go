package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
)

type Config interface {
	ConfigureAws() error
	GetConfig() *aws.Config
}
