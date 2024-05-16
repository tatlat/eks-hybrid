package main

import (
	"github.com/integrii/flaggy"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/cmd/nodeadm/config"
	initcmd "github.com/aws/eks-hybrid/cmd/nodeadm/init"
	"github.com/aws/eks-hybrid/cmd/nodeadm/install"
	"github.com/aws/eks-hybrid/internal/cli"
)

func main() {
	flaggy.SetName("nodeadm")
	flaggy.SetDescription("From zero to Node faster than you can say Elastic Kubernetes Service")
	flaggy.DefaultParser.AdditionalHelpPrepend = "\nhttp://github.com/aws/eks-hybrid"
	flaggy.DefaultParser.ShowHelpOnUnexpected = true

	opts := cli.NewGlobalOptions()

	cmds := []cli.Command{
		config.NewConfigCommand(),
		initcmd.NewInitCommand(),
		install.NewCommand(),
	}

	for _, cmd := range cmds {
		flaggy.AttachSubcommand(cmd.Flaggy(), 1)
	}
	flaggy.Parse()

	log := cli.NewLogger(opts)

	for _, cmd := range cmds {
		if cmd.Flaggy().Used {
			err := cmd.Run(log, opts)
			if err != nil {
				log.Fatal("Command failed", zap.Error(err))
			}
			return
		}
	}
	flaggy.ShowHelpAndExit("No command specified")
}
