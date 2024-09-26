package cmd

import (
	"fmt"

	"github.com/integrii/flaggy"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/internal/test/e2e"
)

type command struct {
	flaggy        *flaggy.Subcommand
	clusterConfig e2e.ClusterConfig
}

func NewSetupCommand() cli.Command {
	cmd := command{}

	setupCmd := flaggy.NewSubcommand("setup")
	setupCmd.Description = "Setup E2E test architecture"
	setupCmd.AdditionalHelpPrepend = "This command will run the setup architecture for running E2E tests"

	setupCmd.String(&cmd.clusterConfig.ClusterName, "c", "clustername", "Name of the EKS cluster")
	setupCmd.String(&cmd.clusterConfig.ClusterRegion, "r", "region", "AWS region where the cluster is created")
	setupCmd.StringSlice(&cmd.clusterConfig.KubernetesVersions, "k", "kubernetes-versions", "List of supported kubernetes versions")
	setupCmd.String(&cmd.clusterConfig.ClusterVpcCidr, "v", "cluster-vpc-cidr", "EKS cluster VPC CIDR")
	setupCmd.String(&cmd.clusterConfig.ClusterPublicSubnetCidr, "u", "cluster-public-subnet", "CIDR for public subnet of EKS VPC")
	setupCmd.String(&cmd.clusterConfig.ClusterPrivateSubnetCidr, "s", "cluster-private-subnet", "CIDR for private subnet of EKS VPC")
	setupCmd.String(&cmd.clusterConfig.HybridNodeCidr, "e", "ec2-vpc-cidr", "CIDR for hybrid EC2 VPC")
	setupCmd.String(&cmd.clusterConfig.HybridPodCidr, "p", "ec2-pod-cidr", "Hybrid EC2 instance pod CIDR")
	setupCmd.String(&cmd.clusterConfig.HybridPubicSubnetCidr, "b", "ec2-public-subnet", "CIDR for public subnet of hybrid EC2 VPC")
	setupCmd.String(&cmd.clusterConfig.HybridPrivateSubnetCidr, "i", "ec2-private-cidr", "CIDR for private subnet of hybrid EC2 VPC")
	setupCmd.StringSlice(&cmd.clusterConfig.Networking, "n", "cnis", "List of supported CNIs")

	cmd.flaggy = setupCmd

	return &cmd
}

func (c *command) Flaggy() *flaggy.Subcommand {
	return c.flaggy
}

func (s *command) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	fmt.Println("Starting cluster setup...")

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(s.clusterConfig.ClusterRegion),
	})
	if err != nil {
		return fmt.Errorf("error creating AWS session: %v", err)
	}

	s.clusterConfig.Session = sess

	err = e2e.CreateResources(&s.clusterConfig)
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("error while setting up E2E test architecture: %v", err)
	}

	fmt.Println("Cluster setup completed successfully!")
	return nil
}
