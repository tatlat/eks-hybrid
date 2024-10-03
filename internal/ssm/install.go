package ssm

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	awsSsm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

// installerPath is the path the SSM CLI installer is installed to.
const installerPath = "/opt/aws/ssm-setup-cli"

// Source serves an SSM installer binary for the target platform.
type Source interface {
	GetSSMInstaller(context.Context) (io.ReadCloser, error)
}

// PkgSource serves and defines the package for target platform
type PkgSource interface {
	GetSSMPackage() artifact.Package
}

func Install(ctx context.Context, tracker *tracker.Tracker, source Source) error {
	installer, err := source.GetSSMInstaller(ctx)
	if err != nil {
		return err
	}
	defer installer.Close()

	if err := artifact.InstallFile(installerPath, installer, 0755); err != nil {
		return fmt.Errorf("ssm installer: %w", err)
	}
	if err = tracker.Add(artifact.Ssm); err != nil {
		return err
	}

	return nil
}

// Uninstall de-registers the managed instance and removes all files and components that
// make up the ssm agent component.
func Uninstall(pkgSource PkgSource) error {
	instanceId, region, err := GetManagedHybridInstanceIdAndRegion()

	// If uninstall is being run just after running install and before running init
	// SSM would not be fully installed and registered, hence it's not required to run
	// deregister instance.
	if err != nil && os.IsNotExist(err) {
		return os.RemoveAll(installerPath)
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

	ssmPkg := pkgSource.GetSSMPackage()
	if err := artifact.UninstallPackage(ssmPkg); err != nil {
		return err
	}

	if err := os.Remove(registrationFilePath); err != nil {
		return err
	}

	err = os.RemoveAll(symlinkedAWSConfigPath)
	if err != nil {
		return fmt.Errorf("removing directory %s: %v", symlinkedAWSConfigPath, err)
	}

	return os.RemoveAll(installerPath)
}

// redownloadInstaller deletes and downloads a new ssm installer
func redownloadInstaller(region string) error {
	if err := os.RemoveAll(installerPath); err != nil {
		return err
	}
	trackerConf, err := tracker.GetCurrentState()
	if err != nil {
		return err
	}
	installer := NewSSMInstaller(region)
	if err := Install(context.Background(), trackerConf, installer); err != nil {
		return err
	}
	return trackerConf.Save()
}
