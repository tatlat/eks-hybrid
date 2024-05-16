package ssm

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/eks-hybrid/internal/artifact"
)

// InstallerPath is the path the SSM CLI installer is installed to.
const InstallerPath = "/opt/aws/ssm-setup-cli"

// Source serves an SSM installer binary for the target platform.
type Source interface {
	GetSSMInstaller(context.Context) (io.ReadCloser, error)
}

func Install(ctx context.Context, source Source) error {
	installer, err := source.GetSSMInstaller(ctx)
	if err != nil {
		return err
	}
	defer installer.Close()

	if err := artifact.InstallFile(InstallerPath, installer, 0755); err != nil {
		return fmt.Errorf("ssm installer: %w", err)
	}

	return nil
}
