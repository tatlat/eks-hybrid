package ssm

import (
	"os"
	"os/exec"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/util"
)

// Upgrade will remove the ssm agent on node, and run the installer which will install
// a newer version of ssm and register the node. Upgrade will not deregister the managed node.
// SSM agent will not consume another registration when using upgrade.
func Upgrade(ssmDaemon daemon.Daemon, nodeConfig *api.NodeConfig) error {
	instanceId, err := GetManagedHybridInstanceId()

	// If uninstall is being run just after running install and before running init
	// SSM would not be fully installed and registered, hence we can just remove the
	// old installer
	if err != nil && os.IsNotExist(err) {
		os.RemoveAll(InstallerPath)
	} else if err != nil {
		return err
	}

	// SSM register had a successful run, which mean the agent is running
	// Removing the agent from node
	if instanceId != "" {
		osToRemoveCommand := map[string]*exec.Cmd{
			util.UbuntuOsName: exec.Command("snap", "remove", "amazon-ssm-agent"),
			util.RhelOsName:   exec.Command("yum", "remove", "amazon-ssm-agent", "-y"),
			util.AmazonOsName: exec.Command("yum", "remove", "amazon-ssm-agent", "-y"),
		}
		osName := util.GetOsName()
		if cmd, ok := osToRemoveCommand[osName]; ok {
			if _, err := cmd.CombinedOutput(); err != nil {
				return err
			}
		}
	}

	return ssmDaemon.Configure(nodeConfig)
}
