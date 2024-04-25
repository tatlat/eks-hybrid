package imagecredentialprovider

import (
	"context"
	"fmt"
	"io"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

// BinPath is the path to the image-credential-provider binary.
const BinPath = "/etc/eks/image-credential-provider/image-credential-provider"

// Source represents a source that serves a image-credential-provider binary.
type Source interface {
	GetImageCredentialProvider(context.Context) (io.ReadCloser, error)
}

// Install installs the image-credential-provider at BinPath.
func Install(ctx context.Context, src Source) error {
	imageCredentialProvider, err := src.GetImageCredentialProvider(ctx)
	if err != nil {
		return nil
	}
	defer imageCredentialProvider.Close()

	if err := artifact.VerifyChecksum(imageCredentialProvider); err != nil {
		return fmt.Errorf("image-credential-provider: %w", err)
	}

	return artifact.InstallFile(BinPath, imageCredentialProvider, 0755)
}
