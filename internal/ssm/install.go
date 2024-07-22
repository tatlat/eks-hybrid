package ssm

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
	"github.com/aws/eks-hybrid/internal/util"

	"github.com/aws/aws-sdk-go-v2/config"
	awsSsm "github.com/aws/aws-sdk-go-v2/service/ssm"
)

// InstallerPath is the path the SSM CLI installer is installed to.
const InstallerPath = "/opt/aws/ssm-setup-cli"

// Source serves an SSM installer binary for the target platform.
type Source interface {
	GetSSMInstaller(context.Context) (io.ReadCloser, error)
}

func Install(ctx context.Context, tracker *tracker.Tracker, source Source) error {
	installer, err := source.GetSSMInstaller(ctx)
	if err != nil {
		return err
	}
	defer installer.Close()

	if err := artifact.InstallFile(InstallerPath, installer, 0755); err != nil {
		return fmt.Errorf("ssm installer: %w", err)
	}
	if err = tracker.Add(artifact.Ssm); err != nil {
		return err
	}

	return nil
}

// Uninstall deregisters the managed instance and removes all files and components that
// make up the ssm agent component.
func Uninstall() error {
	instanceId, region, err := GetManagedHybridInstanceIdAndRegion()

	// If uninstall is being run just after running install and before running init
	// SSM would not be fully installed and registered, hence it's not required to run
	// deregister instance.
	if err != nil && os.IsNotExist(err) {
		return os.RemoveAll(InstallerPath)
	} else if err != nil {
		return err
	}

	// Create SSM client
	awsConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return err
	}
	ssmClient := awsSsm.NewFromConfig(awsConfig)
	managed, err := isInstanceManaged(ssmClient, instanceId)
	if err != nil {
		return err
	}

	// Only deregister the instance if init/ssm init was run and
	// if instances is actively listed as managed
	if managed {
		if err := deregister(ssmClient, instanceId); err != nil {
			return err
		}
	}

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

	if err := os.Remove(registrationFilePath); err != nil {
		return err
	}

	return os.RemoveAll(InstallerPath)
}
