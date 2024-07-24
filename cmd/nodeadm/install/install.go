package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/integrii/flaggy"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/aws/eks"
	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/internal/cni"
	"github.com/aws/eks-hybrid/internal/configprovider"
	"github.com/aws/eks-hybrid/internal/containerd"
	"github.com/aws/eks-hybrid/internal/iamauthenticator"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
	"github.com/aws/eks-hybrid/internal/imagecredentialprovider"
	"github.com/aws/eks-hybrid/internal/kubectl"
	"github.com/aws/eks-hybrid/internal/kubelet"
	"github.com/aws/eks-hybrid/internal/ssm"
	"github.com/aws/eks-hybrid/internal/tracker"
)

func NewCommand() cli.Command {
	cmd := command{}

	fc := flaggy.NewSubcommand("install")
	fc.Description = "Install components required to join an EKS cluster"
	fc.AddPositionalValue(&cmd.kubernetesVersion, "KUBERNETES_VERSION", 1, true, "The major[.minor[.patch]] version of Kubernetes to install")

	cmd.flaggy = fc

	return &cmd
}

type command struct {
	flaggy            *flaggy.Subcommand
	kubernetesVersion string
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
	v := api.NewValidator(nodeCfg)
	if err := v.Validate(nodeCfg); err != nil {
		return err
	}

	ctx := context.Background()
	log.Info("Validating Kubernetes version", zap.Reflect("kubernetes version", c.kubernetesVersion))
	// Create a Source for all EKS managed artifacts.
	release, err := eks.FindLatestRelease(ctx, c.kubernetesVersion)
	if err != nil {
		return err
	}
	log.Info("Using Kubernetes version", zap.Reflect("kubernetes version", release.Version))

	return Install(ctx, nodeCfg, release, log)
}

func Install(ctx context.Context, nodeConfig *api.NodeConfig, eksRelease eks.PatchRelease, log *zap.Logger) error {
	// Create tracker with existing changes or new tracker
	trackerConf, err := tracker.GetCurrentState()
	if err != nil {
		return err
	}

	log.Info("Installing containerd...")
	if err := containerd.Install(trackerConf); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	if err := containerd.ValidateSystemdUnitFile(); err != nil {
		return fmt.Errorf("please install systemd unit file for containerd: %v", err)
	}

	switch {
	case nodeConfig.IsIAMRolesAnywhere():
		signingHelper := iamrolesanywhere.NewSigningHelper()

		log.Info("Installing AWS signing helper...")
		if err := iamrolesanywhere.Install(ctx, trackerConf, signingHelper); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	case nodeConfig.IsSSM():
		ssmInstaller := ssm.NewSSMInstaller(nodeConfig.Spec.Cluster.Region)

		log.Info("Installing SSM agent installer...")
		if err := ssm.Install(ctx, trackerConf, ssmInstaller); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	default:
		return errors.New("unable to detect hybrid auth method")
	}

	log.Info("Installing kubelet...")
	if err := kubelet.Install(ctx, trackerConf, eksRelease); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Installing kubectl...")
	if err := kubectl.Install(ctx, trackerConf, eksRelease); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Installing cni-plugins...")
	if err := cni.Install(ctx, trackerConf, eksRelease); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Installing image credential provider...")
	if err := imagecredentialprovider.Install(ctx, trackerConf, eksRelease); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Installing IAM authenticator...")
	if err := iamauthenticator.Install(ctx, trackerConf, eksRelease); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	log.Info("Finishing up install...")
	return trackerConf.Save()
}
