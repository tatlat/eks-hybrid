package cni

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

const (
	// BinPath is the path to the cni plugins binary.
	BinPath = "/opt/cni/bin"

	// TgzPath is the path to install the cni-plugins tgz file
	TgzPath = "/opt/cni/plugins/cni-plugins.tgz"
)

// Source represents a source that serves a cni plugins binary.
type Source interface {
	GetCniPlugins(context.Context) (artifact.Source, error)
}

func Install(ctx context.Context, tracker *tracker.Tracker, src Source) error {
	cniPlugins, err := src.GetCniPlugins(ctx)
	if err != nil {
		return fmt.Errorf("cni-plugins: %w", err)
	}
	defer cniPlugins.Close()

	if err := artifact.InstallFile(TgzPath, cniPlugins, 0755); err != nil {
		return fmt.Errorf("cni-plugins: %w", err)
	}
	if err = tracker.Add(artifact.CniPlugins); err != nil {
		return err
	}

	if !cniPlugins.VerifyChecksum() {
		return fmt.Errorf("cni-plugins: %w", artifact.NewChecksumError(cniPlugins))
	}

	if err := artifact.InstallTarGz(BinPath, TgzPath); err != nil {
		return fmt.Errorf("cni-plugins: %w", err)
	}
	return nil
}

func Uninstall() error {
	return os.RemoveAll(BinPath)
}
