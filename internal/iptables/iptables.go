package iptables

import (
	"os/exec"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

const iptablesBinName = "iptables"

// Source interface for iptables package
type Source interface {
	GetIptables() artifact.Package
}

// Install iptables package required for kubelet
func Install(tracker *tracker.Tracker, source Source) error {
	if !isIptablesInstalled() {
		iptablesSrc := source.GetIptables()
		if err := artifact.InstallPackage(iptablesSrc); err != nil {
			return err
		}
		return tracker.Add(artifact.Iptables)
	}
	return nil
}

// Uninstall iptables package
func Uninstall(source Source) error {
	if isIptablesInstalled() {
		iptablesSrc := source.GetIptables()
		if err := artifact.UninstallPackage(iptablesSrc); err != nil {
			return err
		}
	}
	return nil
}

func isIptablesInstalled() bool {
	_, err := exec.LookPath(iptablesBinName)
	return err == nil
}
