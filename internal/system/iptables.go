package system

import (
	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/util"
)

func InstallIptables() error {
	osName := util.GetOsName()
	if osName == util.AmazonOsName {
		// AL2023 doesn't come with iptables installed
		if err := artifact.InstallPackage("iptables", util.YumPackageManager, true); err != nil {
			return err
		}
	}
	return nil
}
