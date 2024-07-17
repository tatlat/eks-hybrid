package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type advanced struct {
	region      string
	profilePath string
	profileName string
}

func newAdvancedConfig(region, profilePath, profileName string) Config {
	return &advanced{
		region:      region,
		profilePath: profilePath,
		profileName: profileName,
	}
}

func (ac *advanced) GetConfig() (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(ac.region),
		config.WithSharedConfigFiles([]string{ac.profilePath}),
		config.WithSharedConfigProfile(ac.profileName))
	if err != nil {
		return aws.Config{}, err
	}
	return awsConfig, nil
}
