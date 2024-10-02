package cmd

import (
	"fmt"

	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
)

type command struct {
	flaggy *flaggy.Subcommand
}

func NewCleanupCommand() cli.Command {
	cmd := command{}

	cleanup := flaggy.NewSubcommand("cleanup")
	cleanup.Description = "Cleaning up E2E test architecture"
	cleanup.AdditionalHelpPrepend = "This command will cleanup E2E test architecture."

	cmd.flaggy = cleanup

	return &cmd
}

func (c *command) Flaggy() *flaggy.Subcommand {
	return c.flaggy
}

func (s *command) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	fmt.Println("Cleaning up E2E resources...")
	// clean up logic
	return nil
}
