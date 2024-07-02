package kubectl

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

// BinPath is the path to the kubectl binary.
const BinPath = "/usr/local/bin/kubectl"

// Source represents a source that serves a kubectl binary.
type Source interface {
	GetKubectl(context.Context) (artifact.Source, error)
}

// Install installs kubectl at BinPath.
func Install(ctx context.Context, tracker *tracker.Tracker, src Source) error {
	kubectl, err := src.GetKubectl(ctx)
	if err != nil {
		return fmt.Errorf("kubectl: %w", err)
	}
	defer kubectl.Close()

	if err := artifact.InstallFile(BinPath, kubectl, 0755); err != nil {
		return fmt.Errorf("kubectl: %w", err)
	}
	if err = tracker.Add(artifact.Kubectl); err != nil {
		return err
	}

	if !kubectl.VerifyChecksum() {
		return fmt.Errorf("kubectl: %w", artifact.NewChecksumError(kubectl))
	}

	return nil
}

func Uninstall() error {
	return os.RemoveAll(BinPath)
}
