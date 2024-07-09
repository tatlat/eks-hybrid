package imagecredentialprovider

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

// BinPath is the path to the image-credential-provider binary.
const BinPath = "/etc/eks/image-credential-provider/ecr-credential-provider"

// Source represents a source that serves an image-credential-provider binary.
type Source interface {
	GetImageCredentialProvider(context.Context) (artifact.Source, error)
}

// Install installs the image-credential-provider at BinPath.
func Install(ctx context.Context, tracker *tracker.Tracker, src Source) error {
	imageCredentialProvider, err := src.GetImageCredentialProvider(ctx)
	if err != nil {
		return fmt.Errorf("image-credential-provider: %w", err)
	}
	defer imageCredentialProvider.Close()

	if err := artifact.InstallFile(BinPath, imageCredentialProvider, 0755); err != nil {
		return fmt.Errorf("image-credential-provider: %w", err)
	}
	if err = tracker.Add(artifact.ImageCredentialProvider); err != nil {
		return err
	}

	if !imageCredentialProvider.VerifyChecksum() {
		return fmt.Errorf("image-credential-provider: %w", artifact.NewChecksumError(imageCredentialProvider))
	}

	return nil
}

func Uninstall() error {
	return os.RemoveAll(BinPath)
}
