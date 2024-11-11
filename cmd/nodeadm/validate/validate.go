package validate

import (
	"fmt"

	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/internal/configprovider"
	"github.com/aws/eks-hybrid/internal/validation/logger"
	"github.com/aws/eks-hybrid/internal/validation/validator"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
)

func NewCommand() cli.Command {
	validate := validateCmd{}
	validate.cmd = flaggy.NewSubcommand("validate")
	validate.cmd.String(&validate.configSource, "c", "config-source", "Source of node configuration. The format is a URI with supported schemes: [file, imds].")
	validate.cmd.Description = "Validate the node can join an EKS cluster"
	return &validate
}

type validateCmd struct {
	cmd          *flaggy.Subcommand
	configSource string
}

func (c *validateCmd) Flaggy() *flaggy.Subcommand {
	return c.cmd
}

func (c *validateCmd) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	logger.Init()

	logger.Info(fmt.Sprintf("Loading node configuration: %s", c.configSource))
	provider, err := configprovider.BuildConfigProvider(c.configSource)
	if err != nil {
		return err
	}
	nodeConfig, err := provider.Provide()
	if err != nil {
		return err
	}
	logger.Info("Loaded configuration")

	regionCode := nodeConfig.Spec.Cluster.Region
	logger.Info(fmt.Sprintf("Running validations for region: %s", regionCode))

	runner := validator.NewRunner()

	runner.Register(
		validator.DefaultNumCPU(),
		validator.DefaultSysMem(),
		validator.DefaultDiskSize(),
		validator.DefaultNodeOS(),
		validator.DefaultArch(),
		validator.DefaultSysD(),
		validator.DefaultEndpoints(regionCode),
	)

	errs := runner.Run()
	if errs != nil {
		errorMsg := fmt.Sprintf("Following validations failed with warnings or error: %v ", errs.Error())
		logger.Info(errorMsg)
		return fmt.Errorf("validate command failed with warnings or error")
	}
	return nil
}
