package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type defaultConfig struct {
	region string
}

func newDefaultConfig(region string) Config {
	return &defaultConfig{region: region}
}

func (dc *defaultConfig) GetConfig() (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(dc.region))
	if err != nil {
		return aws.Config{}, err
	}
	return awsConfig, nil
}
