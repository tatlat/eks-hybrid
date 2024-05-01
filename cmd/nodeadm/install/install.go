package install

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

func NewCommand() cli.Command {
	cmd := command{}

	fc := flaggy.NewSubcommand("install")
	fc.Description = "Install components required to join an EKS cluster"
	fc.AddPositionalValue(&cmd.kubernetesVersion, "KUBERNETES_VERSION", 1, true, "The major[.minor[.patch]] version of Kubernetes to install")
	fc.String(&cmd.awsConfig, "", "aws-config", "An aws config path to use for hybrid configuration (/etc/aws/hybrid/config)")

	cmd.flaggy = fc

	return &cmd
}

type command struct {
	flaggy            *flaggy.Subcommand
	kubernetesVersion string
	awsConfig         string
}

func (c *command) Flaggy() *flaggy.Subcommand {
	return c.flaggy
}

func (c *command) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	root, err := cli.IsRunningAsRoot()
	if err != nil {
		return err
	}
	if !root {
		return cli.ErrMustRunAsRoot
	}

	// Apply defaults
	if c.awsConfig == "" {
		c.awsConfig = iamrolesanywhere.DefaultAWSConfigPath
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

	// Install the signing helper first because we need to auth when retrieving EKS artifacts
	// via S3 APIs.
	httpClient := http.Client{Timeout: 120 * time.Second}
	signingHelper := iamrolesanywhere.SigningHelper(httpClient)

	if err := iamrolesanywhere.InstallSigningHelper(ctx, signingHelper); err != nil {
		return err
	}

	// Create a Source for all EKS managed artifacts.
	latest, err := eks.FindLatestRelease(ctx, s3.NewFromConfig(awsCfg), c.kubernetesVersion)
	if err != nil {
		return err
	}

	if err := kubelet.Install(ctx, latest); err != nil {
		return err
	}

	if err := kubectl.Install(ctx, latest); err != nil {
		return err
	}

	if err := imagecredentialprovider.Install(ctx, latest); err != nil {
		return err
	}

	if err := iamrolesanywhere.InstallIAMAuthenticator(ctx, latest); err != nil {
		return err
	}

	return nil
}
