package containerd

import (
	"fmt"
	"os/exec"
	"regexp"

	"go.uber.org/zap"
)

func GetContainerdVersion() (string, error) {
	rawVersion, err := GetContainerdVersionRaw()
	if err != nil {
		return "", err
	}
	semVerRegex := regexp.MustCompile(`[0-9]+\.[0-9]+.[0-9]+`)
	return semVerRegex.FindString(string(rawVersion)), nil
}

func GetContainerdVersionRaw() ([]byte, error) {
	zap.L().Info("Reading containerd version from executable")
	output, err := exec.Command("containerd", "--version").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get containerd version: %w", err)
	}
	return output, nil
}
