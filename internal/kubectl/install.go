package kubectl

import (
	"context"
	"fmt"
	"io"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

// BinPath is the path to the kubectl binary.
const BinPath = "/usr/local/bin/kubectl"

// Source represents a source that serves a kubectl binary.
type Source interface {
	GetKubectl(context.Context) (io.ReadCloser, error)
}

// Install installs kubectl at BinPath.
func Install(ctx context.Context, src Source) error {
	kubectl, err := src.GetKubectl(ctx)
	if err != nil {
		return err
	}
	defer kubectl.Close()

	if err := artifact.VerifyChecksum(kubectl); err != nil {
		return fmt.Errorf("kubectl: %w", err)
	}

	return artifact.InstallFile(BinPath, kubectl, 0755)
}
