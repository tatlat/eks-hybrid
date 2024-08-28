package artifact

import "os/exec"

// Package interface defines a package source
// It defines the install and uninstall command to be executed
type Package interface {
	InstallCmd() *exec.Cmd
	UninstallCmd() *exec.Cmd
}

type packageSource struct {
	installCmd   *exec.Cmd
	uninstallCmd *exec.Cmd
}

func NewPackageSource(installCmd, uninstallCmd *exec.Cmd) Package {
	return &packageSource{
		installCmd:   installCmd,
		uninstallCmd: uninstallCmd,
	}
}

func (ps *packageSource) InstallCmd() *exec.Cmd {
	return ps.installCmd
}

func (ps *packageSource) UninstallCmd() *exec.Cmd {
	return ps.uninstallCmd
}
