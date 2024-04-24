package geteksrelease

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/aws/eks"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/cli"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
)

type getEKSReleaseCmd struct {
	flaggyCmd         *flaggy.Subcommand
	kubernetesVersion string
}

func NewCommand() cli.Command {
	cmd := getEKSReleaseCmd{}

	// Build the Flaggy command.
	fc := flaggy.NewSubcommand("get-eks-release")
	fc.Description = "Dump all artifacts for a given EKS release"
	fc.String(&cmd.kubernetesVersion, "k", "kubernetes-version", "the kubernetes version in the form major.minor.patch")

	cmd.flaggyCmd = fc

	return &cmd
}

func (c *getEKSReleaseCmd) Flaggy() *flaggy.Subcommand {
	return c.flaggyCmd
}

func (c *getEKSReleaseCmd) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	ctx := context.Background()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	latest, err := eks.FindLatestRelease(ctx, s3.NewFromConfig(awsCfg), c.kubernetesVersion)
	if err != nil {
		return err
	}

	return eks.ShowReleaseArtifacts(ctx, latest)
}
