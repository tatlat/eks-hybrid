package imagecredentialprovider

import (
	"context"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

// BinPath is the path to the image-credential-provider binary.
const BinPath = "/etc/eks/image-credential-provider/image-credential-provider"

// Source represents a source that serves a image-credential-provider binary.
type Source interface {
	// GetImageCredentialProvider retrieves the image-credential-provider binary.
	GetImageCredentialProvider(context.Context) (artifact.Source, error)
}

// Install installs the image-credential-provider at BinPath.
func Install(ctx context.Context, src Source) error {
	imageCredentialProvider, err := src.GetImageCredentialProvider(ctx)
	if err != nil {
		return nil
	}
	defer imageCredentialProvider.Close()

	return artifact.InstallFile(BinPath, imageCredentialProvider, 0755)
}
