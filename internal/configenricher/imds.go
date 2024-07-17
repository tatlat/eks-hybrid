package configenricher

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/eks-hybrid/internal/aws/ecr"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/api"
)

type imdsConfigEnricher struct {
	logger *zap.Logger
}

func newImdsConfigEnricher(logger *zap.Logger) ConfigEnricher {
	return &imdsConfigEnricher{logger: logger}
}

func (ice *imdsConfigEnricher) Enrich(cfg *api.NodeConfig) error {
	ice.logger.Info("Fetching instance details..")
	imdsClient := imds.New(imds.Options{})
	awsConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithClientLogMode(aws.LogRetries), config.WithEC2IMDSRegion(func(o *config.UseEC2IMDSRegion) {
		o.Client = imdsClient
	}))
	if err != nil {
		return err
	}
	instanceDetails, err := api.GetInstanceDetails(context.TODO(), imdsClient, ec2.NewFromConfig(awsConfig))
	if err != nil {
		return err
	}
	cfg.Status.Instance = *instanceDetails
	ice.logger.Info("Instance details populated", zap.Reflect("details", instanceDetails))
	region := instanceDetails.Region
	ice.logger.Info("Fetching default options...")
	eksRegistry, err := ecr.GetEKSRegistry(region)
	if err != nil {
		return err
	}
	cfg.Status.Defaults = api.DefaultOptions{
		SandboxImage: eksRegistry.GetSandboxImage(),
	}
	ice.logger.Info("Default options populated", zap.Reflect("defaults", cfg.Status.Defaults))
	return nil
}
