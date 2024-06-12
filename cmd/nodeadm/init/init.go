package init

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
	"k8s.io/utils/strings/slices"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/aws/ecr"
	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/internal/configprovider"
	"github.com/aws/eks-hybrid/internal/containerd"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/kubelet"
	"github.com/aws/eks-hybrid/internal/ssm"
	"github.com/aws/eks-hybrid/internal/system"
)

const (
	configPhase = "config"
	runPhase    = "run"
)

func NewInitCommand() cli.Command {
	init := initCmd{}
	init.cmd = flaggy.NewSubcommand("init")
	init.cmd.StringSlice(&init.daemons, "d", "daemon", "specify one or more of `containerd` and `kubelet`. This is intended for testing and should not be used in a production environment.")
	init.cmd.StringSlice(&init.skipPhases, "s", "skip", "phases of the bootstrap you want to skip")
	init.cmd.Description = "Initialize this instance as a node in an EKS cluster"
	return &init
}

type initCmd struct {
	cmd        *flaggy.Subcommand
	skipPhases []string
	daemons    []string
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

	log.Info("Loading configuration..", zap.String("configSource", opts.ConfigSource))
	provider, err := configprovider.BuildConfigProvider(opts.ConfigSource)
	if err != nil {
		return err
	}
	nodeConfig, err := provider.Provide()
	if err != nil {
		return err
	}
	log.Info("Loaded configuration", zap.Reflect("config", nodeConfig))

	log.Info("Enriching configuration..")
	if err := enrichConfig(log, nodeConfig); err != nil {
		return err
	}

	zap.L().Info("Validating configuration..")
	if err := api.ValidateNodeConfig(nodeConfig); err != nil {
		return err
	}

	if nodeConfig.IsHybridNode() {
		// validate and/or create aws config with the inputs from IAM roles anywhere inputs
		// skip this for SSM, as SSM agents generates and owns the aws config.
		if nodeConfig.IsIAMRolesAnywhere() {
			if nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath == "" {
				nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath = iamrolesanywhere.DefaultAWSConfigPath
			}
			if err := iamrolesanywhere.EnsureAWSConfig(iamrolesanywhere.AWSConfig{
				TrustAnchorARN: nodeConfig.Spec.Hybrid.IAMRolesAnywhere.TrustAnchorARN,
				ProfileARN:     nodeConfig.Spec.Hybrid.IAMRolesAnywhere.ProfileARN,
				RoleARN:        nodeConfig.Spec.Hybrid.IAMRolesAnywhere.RoleARN,
				Region:         nodeConfig.Spec.Cluster.Region,
				ConfigPath:     nodeConfig.Spec.Hybrid.IAMRolesAnywhere.AwsConfigPath,
			}); err != nil {
				return err
			}
		}
	}

	log.Info("Creating daemon manager..")
	daemonManager, err := daemon.NewDaemonManager()
	if err != nil {
		return err
	}
	defer daemonManager.Close()

	aspects := []system.SystemAspect{
		system.NewLocalDiskAspect(),
		system.NewNetworkingAspect(),
	}

	var daemons []daemon.Daemon
	// If Hybrid w/ SSM is enabled, we need to make sure SSM daemon is configured first
	// in order to register the instance first. This will provide us both aws credentials
	// and managed instance ID which will override hostname in both kubelet configs & provider-id
	if nodeConfig.IsSSM() {
		daemons = append(daemons, ssm.NewSsmDaemon(daemonManager))
	}

	daemons = append(daemons,
		containerd.NewContainerdDaemon(daemonManager),
		kubelet.NewKubeletDaemon(daemonManager),
	)

	if !slices.Contains(c.skipPhases, configPhase) {
		log.Info("Configuring daemons...")
		for _, daemon := range daemons {
			if len(c.daemons) > 0 && !slices.Contains(c.daemons, daemon.Name()) {
				continue
			}
			nameField := zap.String("name", daemon.Name())

			log.Info("Configuring daemon...", nameField)
			if err := daemon.Configure(nodeConfig); err != nil {
				return err
			}
			log.Info("Configured daemon", nameField)

			// Check if SSM daemon and set node name
			if daemon.Name() == ssm.SsmDaemonName {
				registeredNodeName, err := ssm.GetManagedHybridInstanceId()
				if err != nil {
					return err
				}
				nodeNameField := zap.String("registered instance-id", registeredNodeName)
				log.Info("Re-setting node name with registered managed instance id", nodeNameField)
				nodeConfig.Spec.Hybrid.NodeName = registeredNodeName
			}
		}
	}

	if !slices.Contains(c.skipPhases, runPhase) {
		// Aspects are not required for hybrid nodes
		// Setting up aspects fall under user responsibility for hybrid nodes
		if !nodeConfig.IsHybridNode() {
			log.Info("Setting up system aspects...")
			for _, aspect := range aspects {
				nameField := zap.String("name", aspect.Name())
				log.Info("Setting up system aspect..", nameField)
				if err := aspect.Setup(nodeConfig); err != nil {
					return err
				}
				log.Info("Set up system aspect", nameField)
			}
		}
		for _, daemon := range daemons {
			if len(c.daemons) > 0 && !slices.Contains(c.daemons, daemon.Name()) {
				continue
			}

			nameField := zap.String("name", daemon.Name())

			log.Info("Ensuring daemon is running..", nameField)
			if err := daemon.EnsureRunning(); err != nil {
				return err
			}
			log.Info("Daemon is running", nameField)

			log.Info("Running post-launch tasks..", nameField)
			if err := daemon.PostLaunch(nodeConfig); err != nil {
				return err
			}
			log.Info("Finished post-launch tasks", nameField)
		}
	}

	return nil
}

// Various initializations and verifications of the NodeConfig and
// perform in-place updates when allowed by the user
func enrichConfig(log *zap.Logger, cfg *api.NodeConfig) error {
	var err error
	var eksRegistry ecr.ECRRegistry
	if cfg.IsHybridNode() {
		eksRegistry, err = ecr.GetEKSHybridRegistry(cfg.Spec.Cluster.Region)
		if err != nil {
			return err
		}
	} else {
		log.Info("Fetching instance details..")
		imdsClient := imds.New(imds.Options{})
		awsConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithClientLogMode(aws.LogRetries), config.WithEC2IMDSRegion(func(o *config.UseEC2IMDSRegion) {
			o.Client = imdsClient
		}))
		if err != nil {
			return err
		}
		instanceDetails, err := api.GetInstanceDetails(context.TODO(), imdsClient, ec2.NewFromConfig(awsConfig))
		if err != nil {
			return err
		}
		cfg.Status.Instance = *instanceDetails
		log.Info("Instance details populated", zap.Reflect("details", instanceDetails))
		region := instanceDetails.Region
		log.Info("Fetching default options...")
		eksRegistry, err = ecr.GetEKSRegistry(region)
		if err != nil {
			return err
		}
	}
	cfg.Status.Defaults = api.DefaultOptions{
		SandboxImage: eksRegistry.GetSandboxImage(),
	}
	log.Info("Default options populated", zap.Reflect("defaults", cfg.Status.Defaults))
	return nil
}
