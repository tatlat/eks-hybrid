package install

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/ptr"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/aws/eks"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/cli"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/configprovider"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/iamrolesanywhere"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/imagecredentialprovider"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/kubectl"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/kubelet"
)

func NewInstallCommand() cli.Command {
	install := installCmd{}
	install.cmd = flaggy.NewSubcommand("install")
	install.cmd.Description = "Install components required to join an EKS cluster"
	install.cmd.String(&install.kubernetesVersion, "k", "kubernetes-version", "the kubernetes major and minor version to install")
	install.cmd.String(&install.awsConfig, "", "aws-config", "the aws config path")
	return &install
}

type installCmd struct {
	cmd               *flaggy.Subcommand
	kubernetesVersion string
	awsConfig         string
}

func (c *installCmd) Flaggy() *flaggy.Subcommand {
	return c.cmd
}

func (c *installCmd) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	root, err := cli.IsRunningAsRoot()
	if err != nil {
		return err
	}
	if !root {
		return cli.ErrMustRunAsRoot
	}

	log.Info("Loading configuration", zap.String("configSource", opts.ConfigSource))
	provider, err := configprovider.BuildConfigProvider(opts.ConfigSource)
	if err != nil {
		return err
	}
	nodeCfg, err := provider.Provide()
	if err != nil {
		return err
	}
	log.Info("Loaded configuration", zap.Reflect("config", nodeCfg))

	// Ensure hybrid configuration

	log.Info("Validating configuration")
	if err := api.ValidateNodeConfig(nodeCfg); err != nil {
		return err
	}

	if err := iamrolesanywhere.EnsureAWSConfig(iamrolesanywhere.AWSConfig{
		TrustAnchorARN: nodeCfg.Spec.Hybrid.IAMRolesAnywhere.TrustAnchorARN,
		ProfileARN:     nodeCfg.Spec.Hybrid.IAMRolesAnywhere.ProfileARN,
		RoleARN:        nodeCfg.Spec.Hybrid.IAMRolesAnywhere.RoleARN,
		Region:         nodeCfg.Spec.Hybrid.Region,
		ConfigPath:     c.awsConfig,
	}); err != nil {
		return err
	}

	ctx := context.Background()

	log.Info("Loading configuration")
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigFiles([]string{c.awsConfig}),
		config.WithSharedConfigProfile(iamrolesanywhere.ProfileName))
	if err != nil {
		return err
	}

	eksClient := awseks.NewFromConfig(awsCfg)
	clstr, err := eksClient.DescribeCluster(ctx, &awseks.DescribeClusterInput{
		Name: ptr.String(nodeCfg.Spec.Cluster.Name),
	})
	if err != nil {
		return err
	}

	// Create a Source for all EKS managed artifacts.
	latest, err := eks.FindLatestRelease(ctx, s3.NewFromConfig(awsCfg),
		*clstr.Cluster.Version)
	if err != nil {
		return err
	}

	// Create a Source for retrieving the signing helper.
	httpClient := http.Client{Timeout: 120 * time.Second}
	signingHelper := iamrolesanywhere.SigningHelper(httpClient)

	if err := kubelet.Install(ctx, latest); err != nil {
		return err
	}

	if err := kubectl.Install(ctx, latest); err != nil {
		return err
	}

	if err := imagecredentialprovider.Install(ctx, latest); err != nil {
		return err
	}

	if err := iamrolesanywhere.Install(ctx, signingHelper, latest); err != nil {
		return err
	}

	return nil
}
