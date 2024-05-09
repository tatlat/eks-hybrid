package kubectl

import (
	"context"
	"fmt"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

// BinPath is the path to the kubectl binary.
const BinPath = "/usr/local/bin/kubectl"

// Source represents a source that serves a kubectl binary.
type Source interface {
	GetKubectl(context.Context) (artifact.Source, error)
}

// Install installs kubectl at BinPath.
func Install(ctx context.Context, src Source) error {
	kubectl, err := src.GetKubectl(ctx)
	if err != nil {
		return fmt.Errorf("kubectl: %w", err)
	}
	defer kubectl.Close()

	if err := artifact.InstallFile(BinPath, kubectl, 0755); err != nil {
		return fmt.Errorf("kubectl: %w", err)
	}

	if !kubectl.VerifyChecksum() {
		return fmt.Errorf("kubectl: %w", artifact.NewChecksumError(kubectl))
	}

	return nil
}
