package config

import (
	"github.com/aws/eks-hybrid/internal/cli"
)

func NewConfigCommand() cli.Command {
	container := cli.NewCommandContainer("config", "Manage configuration")
	container.AddCommand(NewCheckCommand())
	return container.AsCommand()
}
