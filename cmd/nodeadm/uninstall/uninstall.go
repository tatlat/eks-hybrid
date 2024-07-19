package uninstall

import (
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
	"os"

	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/internal/cni"
	"github.com/aws/eks-hybrid/internal/daemon"
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

	fc := flaggy.NewSubcommand("uninstall")
	fc.Description = "Uninstall components installed using the install sub-command"
	cmd.flaggy = fc

	return &cmd
}

type command struct {
	flaggy *flaggy.Subcommand
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

	log.Info("Loading installed components")
	installed, err := tracker.GetInstalledArtifacts()
	if err != nil && os.IsNotExist(err) {
		log.Info("Nodeadm components are already uninstalled")
		return nil
	} else if err != nil {
		return err
	}

	log.Info("Creating daemon manager..")
	daemonManager, err := daemon.NewDaemonManager()
	if err != nil {
		return err
	}
	defer daemonManager.Close()

	artifacts := installed.Artifacts
	if artifacts.Kubectl {
		log.Info("Uninstalling kubectl...")
		if err := kubectl.Uninstall(); err != nil {
			return err
		}
	}
	if artifacts.CniPlugins {
		log.Info("Uninstalling cni-plugins...")
		if err := cni.Uninstall(); err != nil {
			return err
		}
	}
	if artifacts.IamAuthenticator {
		log.Info("Uninstalling IAM authenticator...")
		if err := iamauthenticator.Uninstall(); err != nil {
			return err
		}
	}
	if artifacts.IamRolesAnywhere {
		log.Info("Uninstalling AWS signing helper...")
		if err := iamrolesanywhere.Uninstall(); err != nil {
			return err
		}
	}
	if artifacts.ImageCredentialProvider {
		log.Info("Uninstalling image credential provider...")
		if err := imagecredentialprovider.Uninstall(); err != nil {
			return err
		}
	}
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
		log.Info("Uninstalling and de-registering SSM agent...")
		ssmDaemon := ssm.NewSsmDaemon(daemonManager)
		if err := ssmDaemon.Stop(); err != nil {
			return err
		}
		if err := ssm.Uninstall(); err != nil {
			return err
		}
	}

	return tracker.Clear()
}
