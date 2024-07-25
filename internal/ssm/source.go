package ssm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// Initial region ssm installer is downloaded from. When installer runs, it will
// down the agent from the proper region configured in the nodeConfig during init command
const ssmInstallerRegion = "us-west-2"

// SSMInstaller provides a Source that retrieves the SSM installer from the official
// release endpoint.
func NewSSMInstaller() Source {
	return ssmInstallerSource{
		client: http.Client{Timeout: 120 * time.Second},
	}
}

type ssmInstallerSource struct {
	client http.Client
}

func (s ssmInstallerSource) GetSSMInstaller(ctx context.Context) (io.ReadCloser, error) {
	endpoint, err := buildSSMURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func buildSSMURL() (string, error) {
	variant, err := detectPlatformVariant()
	if err != nil {
		return "", err
	}

	platform := fmt.Sprintf("%v_%v", variant, runtime.GOARCH)
	return fmt.Sprintf("https://amazon-ssm-%v.s3.%v.amazonaws.com/latest/%v/ssm-setup-cli", ssmInstallerRegion, ssmInstallerRegion, platform), nil
}

// detectPlatformVariant returns a portion of the SSM installers URL that is dependent on the
// package manager in use.
func detectPlatformVariant() (string, error) {
	toVariant := map[string]string{
		"apt": "debian",
		"dnf": "linux",
		"yum": "linux",
	}

	for pkgManager := range toVariant {
		_, err := exec.LookPath(pkgManager)
		if errors.Is(err, exec.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}

		return toVariant[pkgManager], nil
	}

	return "", errors.New("unsupported platform")
}
