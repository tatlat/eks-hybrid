package ssm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
)

// SSMInstaller provides a Source that retrieves the SSM installer from the official
// release endpoint.
func SSMInstaller(client http.Client, region string) Source {
	return ssmInstallerSource{
		client: client,
		region: region,
	}
}

type ssmInstallerSource struct {
	client http.Client
	region string
}

func (s ssmInstallerSource) GetSSMInstaller(ctx context.Context) (io.ReadCloser, error) {
	endpoint, err := buildSSMURL(s.region)
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

func buildSSMURL(region string) (string, error) {
	variant, err := detectPlatformVariant()
	if err != nil {
		return "", err
	}

	platform := fmt.Sprintf("%v_%v", variant, runtime.GOARCH)
	return fmt.Sprintf("https://s3.%v.amazonaws.com/latest/%v/ssm-setup-cli", region, platform), nil
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
