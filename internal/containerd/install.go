package containerd

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/tracker"
	"github.com/aws/eks-hybrid/internal/util"
)

const (
	containerdPackageName = "containerd"
	containerdBinPath     = "/usr/bin/containerd"
	runcBinPath           = "/usr/bin/runc"
)

// Install installs containerd on the node using package manager
// todo: Installs using docker builds with source options
func Install(tracker *tracker.Tracker) error {
	// check if containerd or runc is already installed
	_, containerdErr := os.Stat(containerdBinPath)
	_, runcErr := os.Stat(runcBinPath)
	if os.IsNotExist(containerdErr) || os.IsNotExist(runcErr) {
		pkgManagers, osName := util.GetOsPackageManagers()
		if osName == util.RhelOsName {
			return fmt.Errorf("installing containerd is not supported on RedHat Os. Please install containerd " +
				"and systemd unit for containerd before running nodeadm")
		}

		for _, manager := range pkgManagers {
			if err := artifact.InstallPackage(containerdPackageName, manager, true); err == nil {
				return tracker.Add(artifact.Containerd)
			}
		}
		return fmt.Errorf("no package managers qualified to install containerd. Please install before running nodeadm")
	}

	return fs.ErrExist
}

func Uninstall() error {
	pkgManagers, _ := util.GetOsPackageManagers()
	for _, manager := range pkgManagers {
		if err := artifact.UninstallPackage(containerdPackageName, manager); err == nil {
			return nil
		}
	}
	return fmt.Errorf("No package managers qualified to remove containerd package")
}

func ValidateSystemdUnitFile() error {
	daemonManager, err := daemon.NewDaemonManager()
	if err != nil {
		return err
	}
	if err := daemonManager.DaemonReload(); err != nil {
		return err
	}
	daemonStatus, err := daemonManager.GetDaemonStatus(ContainerdDaemonName)
	if daemonStatus == daemon.DaemonStatusUnknown || err != nil {
		return fmt.Errorf("containerd daemon not found")
	}
	return nil
}
