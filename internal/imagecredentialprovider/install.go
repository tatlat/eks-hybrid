package imagecredentialprovider

import (
	"context"
	"fmt"

	"github.com/aws/eks-hybrid/internal/artifact"
)

// BinPath is the path to the image-credential-provider binary.
const BinPath = "/etc/eks/image-credential-provider/image-credential-provider"

// Source represents a source that serves a image-credential-provider binary.
type Source interface {
	GetImageCredentialProvider(context.Context) (artifact.Source, error)
}

// Install installs the image-credential-provider at BinPath.
func Install(ctx context.Context, src Source) error {
	imageCredentialProvider, err := src.GetImageCredentialProvider(ctx)
	if err != nil {
		return fmt.Errorf("image-credential-provider: %w", err)
	}
	defer imageCredentialProvider.Close()

	if err := artifact.InstallFile(BinPath, imageCredentialProvider, 0755); err != nil {
		return fmt.Errorf("image-credential-provider: %w", err)
	}

	if !imageCredentialProvider.VerifyChecksum() {
		return fmt.Errorf("image-credential-provider: %w", artifact.NewChecksumError(imageCredentialProvider))
	}

	return nil
}
