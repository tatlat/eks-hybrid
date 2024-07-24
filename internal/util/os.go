package util

import (
	"github.com/go-ini/ini"
)

const (
	UbuntuOsName = "ubuntu"
	RhelOsName   = "rhel"
	AmazonOsName = "amzn"

	AptPackageManager  = "apt"
	SnapPackageManager = "snap"
	YumPackageManager  = "yum"
)

// GetOsName reads the /etc/os-release file and returns the os name
func GetOsName() string {
	cfg, _ := ini.Load("/etc/os-release")
	return cfg.Section("").Key("ID").String()
}

func GetOsPackageManagers() ([]string, string) {
	osName := GetOsName()
	packageManagers := map[string][]string{
		UbuntuOsName: {AptPackageManager, SnapPackageManager},
		AmazonOsName: {YumPackageManager},
		RhelOsName:   {YumPackageManager},
	}
	if managers, ok := packageManagers[osName]; ok {
		return managers, osName
	}
	return nil, osName
}
