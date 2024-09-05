package init

import (
	"os"

	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/internal/node"
	"github.com/aws/eks-hybrid/internal/nodeprovider"
	"github.com/aws/eks-hybrid/internal/tracker"

	"github.com/integrii/flaggy"
	"go.uber.org/zap"
)

func NewInitCommand() cli.Command {
	init := initCmd{}
	init.cmd = flaggy.NewSubcommand("init")
	init.cmd.StringSlice(&init.daemons, "d", "daemon", "specify one or more of `containerd` and `kubelet`. This is intended for testing and should not be used in a production environment.")
	init.cmd.Description = "Initialize this instance as a node in an EKS cluster"
	return &init
}

type initCmd struct {
	cmd     *flaggy.Subcommand
	daemons []string
}

func (c *initCmd) Flaggy() *flaggy.Subcommand {
	return c.cmd
}

func (c *initCmd) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	log.Info("Checking user is root..")
	root, err := cli.IsRunningAsRoot()
	if err != nil {
		return err
	} else if !root {
		return cli.ErrMustRunAsRoot
	}

	log.Info("Loading installed components")
	_, err = tracker.GetInstalledArtifacts()
	if err != nil && os.IsNotExist(err) {
		log.Info("Nodeadm components are not installed. Please run `nodeadm install` before running init")
		return nil
	} else if err != nil {
		return err
	}

	nodeProvider, err := node.NewNodeProvider(opts.ConfigSource, log)
	if err != nil {
		return err
	}
	if err := nodeProvider.ValidateConfig(); err != nil {
		return err
	}
	if err := nodeProvider.Enrich(); err != nil {
		return err
	}

	return Init(nodeProvider)
}

func Init(node nodeprovider.NodeProvider) error {
	aspects := node.GetAspects()
	node.Logger().Info("Setting up system aspects...")
	for _, aspect := range aspects {
		nameField := zap.String("name", aspect.Name())
		node.Logger().Info("Setting up system aspect..", nameField)
		if err := aspect.Setup(); err != nil {
			return err
		}
		node.Logger().Info("Set up system aspect", nameField)
	}

	node.Logger().Info("Configuring Pre-process daemons...")
	if err := node.PreProcessDaemon(); err != nil {
		return err
	}

	node.Logger().Info("Configuring Aws...")
	if err := node.ConfigureAws(); err != nil {
		return err
	}

	daemons, err := node.GetDaemons()
	if err != nil {
		return err
	}
	node.Logger().Info("Configuring daemons...")
	for _, daemon := range daemons {
		nameField := zap.String("name", daemon.Name())

		node.Logger().Info("Configuring daemon...", nameField)
		if err := daemon.Configure(); err != nil {
			return err
		}
		node.Logger().Info("Configured daemon", nameField)
	}

	for _, daemon := range daemons {
		nameField := zap.String("name", daemon.Name())

		node.Logger().Info("Ensuring daemon is running..", nameField)
		if err := daemon.EnsureRunning(); err != nil {
			return err
		}
		node.Logger().Info("Daemon is running", nameField)

		node.Logger().Info("Running post-launch tasks..", nameField)
		if err := daemon.PostLaunch(); err != nil {
			return err
		}
		node.Logger().Info("Finished post-launch tasks", nameField)
	}
	node.Cleanup()
	return nil
}
