package main

import (
	"github.com/integrii/flaggy"
	"go.uber.org/zap"

	cleanup "github.com/aws/eks-hybrid/cmd/e2e-test-runner/cleanup"
	setup "github.com/aws/eks-hybrid/cmd/e2e-test-runner/setup"
	"github.com/aws/eks-hybrid/internal/cli"
)

func main() {
	flaggy.SetName("e2e-test-runner")
	flaggy.SetDescription("An E2E test runner for setting up test architecture for E2E tests")

	cmds := []cli.Command{
		setup.NewSetupCommand(),
		cleanup.NewCleanupCommand(),
	}

	for _, cmd := range cmds {
		flaggy.AttachSubcommand(cmd.Flaggy(), 1)
	}

	flaggy.Parse()

	opts := cli.NewGlobalOptions()
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
