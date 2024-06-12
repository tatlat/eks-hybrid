package cni

import (
	"context"
	"fmt"

	"github.com/aws/eks-hybrid/internal/artifact"
)

const (
	// BinPath is the path to the cni plugins binary.
	BinPath = "/opt/cni/plugins"

	// TgzPath is the path to install the cni-plugins tgz file
	TgzPath = "/opt/cni/plugins/cni-plugins.tgz"
)

// Source represents a source that serves a cni plugins binary.
type Source interface {
	GetCniPlugins(context.Context) (artifact.Source, error)
}

func Install(ctx context.Context, src Source) error {
	cniPlugins, err := src.GetCniPlugins(ctx)
	if err != nil {
		return fmt.Errorf("cni-plugins: %w", err)
	}
	defer cniPlugins.Close()

	if err := artifact.InstallFile(TgzPath, cniPlugins, 0755); err != nil {
		return fmt.Errorf("cni-plugins: %w", err)
	}

	if !cniPlugins.VerifyChecksum() {
		return fmt.Errorf("cni-plugins: %w", artifact.NewChecksumError(cniPlugins))
	}

	if err := artifact.InstallTarGz(BinPath, TgzPath); err != nil {
		return fmt.Errorf("cni-plugins: %w", err)
	}
	return nil
}
