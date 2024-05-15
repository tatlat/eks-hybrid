package install

import (
	"context"
	"errors"
	"io/fs"
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
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/ssm"
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

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigFiles([]string{c.awsConfig}),
		config.WithSharedConfigProfile(iamrolesanywhere.ProfileName))
	if err != nil {
		return err
	}

	httpClient := http.Client{Timeout: 120 * time.Second}

	switch {
	case nodeCfg.IsIAMRolesAnywhere():
		signingHelper := iamrolesanywhere.SigningHelper(httpClient)

		log.Info("Installing AWS signing helper...")
		if err := iamrolesanywhere.InstallSigningHelper(ctx, signingHelper); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	case nodeCfg.IsSSM():
		ssmInstaller := ssm.SSMInstaller(httpClient, nodeCfg.Spec.Hybrid.Region)

		log.Info("Installing SSM agent installer...")
		if err := ssm.Install(ctx, ssmInstaller); err != nil {
			return err
		}
	default:
		return errors.New("unable to detect hybrid auth method")
	}

	// Create a Source for all EKS managed artifacts.
	latest, err := eks.FindLatestRelease(ctx, s3.NewFromConfig(awsCfg), c.kubernetesVersion)
	if err != nil {
		return err
	}

	log.Info("Installing kubelet...")
	if err := kubelet.Install(ctx, latest); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Installing kubectl...")
	if err := kubectl.Install(ctx, latest); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Installing image credential provider...")
	if err := imagecredentialprovider.Install(ctx, latest); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Installing IAM authenticator...")
	if err := iamrolesanywhere.InstallIAMAuthenticator(ctx, latest); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	return nil
}
