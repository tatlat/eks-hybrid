package cmd

import (
	"fmt"
	"os"

	"github.com/integrii/flaggy"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/test/e2e"
)

type command struct {
	flaggy         *flaggy.Subcommand
	configFilePath string
}

func NewCommand() cli.Command {
	cmd := command{}

	setupCmd := flaggy.NewSubcommand("setup")
	setupCmd.Description = "Setup E2E test architecture"
	setupCmd.AdditionalHelpPrepend = "This command will run the setup architecture for running E2E tests"

	setupCmd.String(&cmd.configFilePath, "s", "setup-config-path", "Path to setup config file")

	cmd.flaggy = setupCmd

	return &cmd
}

func (c *command) Flaggy() *flaggy.Subcommand {
	return c.flaggy
}

func (s *command) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	file, err := os.ReadFile(s.configFilePath)
	if err != nil {
		return fmt.Errorf("failed to open configuration file: %v", err)
	}

	testRunner := &e2e.TestRunner{}

	if err = yaml.Unmarshal(file, &testRunner); err != nil {
		return fmt.Errorf("failed to unmarshal configuration from YAML: %v", err)
	}

	// Create AWS session
	testRunner.Session, err = testRunner.NewAWSSession()
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %v", err)
	}

	// Create resources using TestRunner object
	if err := testRunner.CreateResources(); err != nil {
		return fmt.Errorf("failed to create resources: %v", err)
	}

	fmt.Println("E2E setup completed successfully!")
	return nil
}
