package upgrade

import (
	"context"
	"os"

	"github.com/integrii/flaggy"
	"go.uber.org/zap"

	initialize "github.com/aws/eks-hybrid/cmd/nodeadm/init"
	"github.com/aws/eks-hybrid/cmd/nodeadm/install"
	"github.com/aws/eks-hybrid/cmd/nodeadm/uninstall"
	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/aws/eks"
	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/internal/configenricher"
	"github.com/aws/eks-hybrid/internal/configprovider"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/kubelet"
	"github.com/aws/eks-hybrid/internal/ssm"
	"github.com/aws/eks-hybrid/internal/tracker"
)

func NewUpgradeCommand() cli.Command {
	cmd := command{}

	fc := flaggy.NewSubcommand("upgrade")
	fc.Description = "Upgrade components installed using the install sub-command"
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

	log.Info("Loading configuration..", zap.String("configSource", opts.ConfigSource))
	provider, err := configprovider.BuildConfigProvider(opts.ConfigSource)
	if err != nil {
		return err
	}
	nodeConfig, err := provider.Provide()
	if err != nil {
		return err
	}
	log.Info("Loaded configuration", zap.Reflect("config", nodeConfig))

	// Ensure hybrid configuration
	log.Info("Validating configuration")
	v := api.NewValidator(nodeConfig)
	if err := v.Validate(nodeConfig); err != nil {
		return err
	}

	log.Info("Enriching configuration..")
	enricher := configenricher.New(log, nodeConfig)
	if err := enricher.Enrich(nodeConfig); err != nil {
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

	log.Info("Loading installed components")
	installed, err := tracker.GetInstalledArtifacts()
	if err != nil && os.IsNotExist(err) {
		log.Info("No nodeadm components installed. Please use nodeadm install and nodeadm init commands to bootstrap a node")
		return nil
	} else if err != nil {
		return err
	}

	artifacts := installed.Artifacts
	if err := uninstall.UninstallBinaries(artifacts, log); err != nil {
		return err
	}

	log.Info("Creating daemon manager..")
	daemonManager, err := daemon.NewDaemonManager()
	if err != nil {
		return err
	}
	defer daemonManager.Close()

	if artifacts.Kubelet {
		log.Info("Uninstalling kubelet...")
		kubeletDaemon := kubelet.NewKubeletDaemon(daemonManager)
		if err := kubeletDaemon.Stop(); err != nil {
			return err
		}
		if err := kubelet.Uninstall(); err != nil {
			return err
		}
	}

	if artifacts.Ssm {
		log.Info("Uninstalling  SSM agent...")
		ssmDaemon := ssm.NewSsmDaemon(daemonManager)
		if err := ssmDaemon.Stop(); err != nil {
			return err
		}
		if err := ssm.UninstallWithoutDeregister(); err != nil {
			return err
		}

	}
	if err := tracker.Clear(); err != nil {
		return err
	}

	// Installing new version of artifacts
	if err := install.Install(ctx, nodeConfig, release, log); err != nil {
		return err
	}

	return initialize.Init(nodeConfig, nil, daemonManager, log)
}
