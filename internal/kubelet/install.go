package kubelet

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

const (
	// BinPath is the path to the Kubelet binary.
	BinPath = "/usr/bin/kubelet"

	// UnitPath is the path to the Kubelet systemd unit file.
	UnitPath = "/etc/systemd/system/kubelet.service"
)

//go:embed kubelet.service
var kubeletUnitFile []byte

// Source represents a source that serves a kubelet binary.
type Source interface {
	GetKubelet(context.Context) (artifact.Source, error)
}

// Install installs kubelet at BinPath and installs a systemd unit file at UnitPath. The systemd
// unit is configured to launch the kubelet binary.
func Install(ctx context.Context, tracker *tracker.Tracker, src Source) error {
	kubelet, err := src.GetKubelet(ctx)
	if err != nil {
		return fmt.Errorf("kubelet: %w", err)
	}
	defer kubelet.Close()

	if err := artifact.InstallFile(BinPath, kubelet, 0755); err != nil {
		return fmt.Errorf("kubelet: %w", err)
	}
	if err = tracker.Add(artifact.Kubelet); err != nil {
		return err
	}

	if !kubelet.VerifyChecksum() {
		return fmt.Errorf("kubelet: %w", artifact.NewChecksumError(kubelet))
	}

	buf := bytes.NewBuffer(kubeletUnitFile)

	if err := artifact.InstallFile(UnitPath, buf, 0644); err != nil {
		return fmt.Errorf("kubelet systemd unit: %w", err)
	}

	return nil
}

func Uninstall() error {
	pathsToRemove := []string{
		BinPath,
		UnitPath,
		kubeconfigPath,
		kubeletConfigRoot,
	}

	for _, path := range pathsToRemove {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}
