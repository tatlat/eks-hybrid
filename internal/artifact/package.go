package artifact

import (
	"context"
	"os/exec"
)

// Package interface defines a package source
// It defines the install and uninstall command to be executed
type Package interface {
	// InstallCmd returns the command to install the package.
	// Every invocation guarantees a new command.
	InstallCmd(context.Context) *exec.Cmd
	// UninstallCmd returns the command to uninstall the package.
	// Every invocation guarantees a new command.
	UninstallCmd(context.Context) *exec.Cmd
}

type packageSource struct {
	installCmd   Cmd
	uninstallCmd Cmd
}

// Cmd represents a command to be executed.
type Cmd struct {
	Path string
	Args []string
}

// NewCmd returns a new Cmd.
func NewCmd(path string, args ...string) Cmd {
	return Cmd{
		Path: path,
		Args: args,
	}
}

func NewPackageSource(installCmd, uninstallCmd Cmd) Package {
	return &packageSource{
		installCmd:   installCmd,
		uninstallCmd: uninstallCmd,
	}
}

func (ps *packageSource) InstallCmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, ps.installCmd.Path, ps.installCmd.Args...)
}

func (ps *packageSource) UninstallCmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, ps.uninstallCmd.Path, ps.uninstallCmd.Args...)
}
