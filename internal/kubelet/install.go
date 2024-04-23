package kubelet

import (
	"bytes"
	"context"
	_ "embed"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

// BinPath is the path to the Kubelet binary.
const BinPath = "/usr/bin/kubelet"

// UnitPath is the path to the Kubelet systemd unit file.
const UnitPath = "/etc/systemd/system/kubelet.service"

//go:embed kubelet.service
var kubeletUnitFile []byte

// Source represents a source that serves a kubelet binary.
type Source interface {
	GetKubelet(context.Context) (artifact.Source, error)
}

// Install installs kubelet at BinPath and installs a systemd unit file at UnitPath. The systemd
// unit is configured to launch the kubelet binary.
func Install(ctx context.Context, src Source) error {
	kubelet, err := src.GetKubelet(ctx)
	if err != nil {
		return err
	}
	defer kubelet.Close()

	if err := artifact.InstallFile(BinPath, kubelet, 0755); err != nil {
		return err
	}

	buf := artifact.NopSourceCloser(bytes.NewBuffer(kubeletUnitFile))

	return artifact.InstallFile(UnitPath, buf, 0644)
}
