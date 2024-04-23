package iamrolesanywhere

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

const signingHelperVersion = "1.1.1"

type signingHelperSource struct {
	client http.Client
}

// SigningHelper provides a SigningHelper that retrieves the binary from a official release
// channels.
func SigningHelper(client http.Client) SigningHelperSource {
	return signingHelperSource{
		client: client,
	}
}

// GetSigningHelper retrieves the aws_sigining_helper for IAM Roles Anywhere.
func (src signingHelperSource) GetSigningHelper(ctx context.Context) (artifact.Source, error) {
	if runtime.GOARCH != "amd64" {
		return artifact.Source{}, fmt.Errorf("signing helper: unsupported architecture: %v", runtime.GOARCH)
	}

	url := fmt.Sprintf("https://rolesanywhere.amazonaws.com/releases/%v/X86_64/Linux/aws_signing_helper", signingHelperVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return artifact.Source{}, err
	}

	resp, err := src.client.Do(req)
	if err != nil {
		return artifact.Source{}, err
	}

	if resp.StatusCode != 200 {
		return artifact.Source{}, fmt.Errorf("signing helper: %v", resp.StatusCode)
	}

	return artifact.Source{ReadCloser: resp.Body}, nil
}
