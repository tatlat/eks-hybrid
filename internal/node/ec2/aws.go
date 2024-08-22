package ec2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/config"
)

func (enp *ec2NodeProvider) ConfigureAws() error {
	region := enp.nodeConfig.Status.Instance.Region
	awsConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return err
	}
	enp.awsConfig = &awsConfig
	return nil
}

func (enp *ec2NodeProvider) GetConfig() *aws.Config {
	return enp.awsConfig
}
