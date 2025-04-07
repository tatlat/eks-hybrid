package kubelet

import (
	"bytes"
	"context"
	_ "embed"
	stdErrors "errors"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

const (
	// BinPath is the path to the Kubelet binary.
	BinPath = "/usr/bin/kubelet"

	// UnitPath is the path to the Kubelet systemd unit file.
	UnitPath = "/etc/systemd/system/kubelet.service"

	artifactName      = "kubelet"
	artifactFilePerms = 0o755
)

var kubeletCurrentCertPath = path.Join(kubeconfigRoot, "pki", "kubelet-server-current.pem")

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
		return errors.Wrap(err, "failed to get kubelet source")
	}
	defer kubelet.Close()

	if err := artifact.InstallFile(BinPath, kubelet, artifactFilePerms); err != nil {
		return errors.Wrap(err, "failed to install kubelet")
	}

	if !kubelet.VerifyChecksum() {
		return errors.Errorf("kubelet checksum mismatch: %v", artifact.NewChecksumError(kubelet))
	}
	if err = tracker.Add(artifact.Kubelet); err != nil {
		return err
	}

	buf := bytes.NewBuffer(kubeletUnitFile)

	if err := artifact.InstallFile(UnitPath, buf, 0o644); err != nil {
		return errors.Errorf("failed to install kubelet systemd unit: %v", err)
	}

	return nil
}

type UninstallOptions struct {
	// InstallRoot is optionally the root directory of the installation
	// If not provided, the default will be /
	InstallRoot string
}

func Uninstall(opts UninstallOptions) error {
	pathsToRemove := []string{
		filepath.Join(opts.InstallRoot, BinPath),
		filepath.Join(opts.InstallRoot, UnitPath),
		filepath.Join(opts.InstallRoot, kubeconfigPath),
		filepath.Join(opts.InstallRoot, path.Dir(kubeletConfigRoot)),
		filepath.Join(opts.InstallRoot, kubeletCurrentCertPath),
	}

	allErrors := []error{}

	// resolve the symlink and add actual file to remove
	actualCertPath, err := filepath.EvalSymlinks(filepath.Join(opts.InstallRoot, kubeletCurrentCertPath))
	if err != nil && !os.IsNotExist(err) {
		allErrors = append(allErrors, errors.Wrap(err, "resolving symlink for kubelet cert"))
	}
	if actualCertPath != "" {
		pathsToRemove = append(pathsToRemove, actualCertPath)
	}

	for _, path := range pathsToRemove {
		if err := os.RemoveAll(path); err != nil {
			allErrors = append(allErrors, err)
		}
	}
	if len(allErrors) > 0 {
		return stdErrors.Join(allErrors...)
	}
	return nil
}

func Upgrade(ctx context.Context, src Source, log *zap.Logger) error {
	kubelet, err := src.GetKubelet(ctx)
	if err != nil {
		return errors.Wrap(err, "getting kubelet source")
	}
	defer kubelet.Close()

	return artifact.Upgrade(artifactName, BinPath, kubelet, artifactFilePerms, log)
}
