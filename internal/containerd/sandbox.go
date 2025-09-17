package containerd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/containerd/containerd/integration/remote"
	"go.uber.org/zap"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/aws/eks-hybrid/internal/aws/ecr"
	"github.com/aws/eks-hybrid/internal/util"
)

var (
	// Config version detection
	containerdConfigVersionRegex = regexp.MustCompile(`version = (\d+)`)

	// Containerd config version-specific sandbox image patterns
	containerdSandboxImageV2Regex = regexp.MustCompile(`sandbox_image = ['"]([^'"]*)['"]`)
	containerdSandboxImageV3Regex = regexp.MustCompile(`sandbox = ['"]([^'"]*)['"]`)
)

func cacheSandboxImage(awsConfig *aws.Config) error {
	zap.L().Info("Looking up current sandbox image in containerd config...")
	// capture the output of a `containerd config dump`, which is the final
	// containerd configuration used after all of the applied transformations
	dump, err := exec.Command("containerd", "config", "dump").Output()
	if err != nil {
		return err
	}

	// Parse config version to choose appropriate regex
	configVersion, err := parseConfigVersion(dump)
	if err != nil {
		return fmt.Errorf("failed to parse containerd config version: %w", err)
	}

	// Choose appropriate regex based on config version
	var sandboxRegex *regexp.Regexp
	switch configVersion {
	case 2:
		sandboxRegex = containerdSandboxImageV2Regex
	case 3:
		sandboxRegex = containerdSandboxImageV3Regex
	default:
		return fmt.Errorf("unsupported containerd config version: %d", configVersion)
	}

	matches := sandboxRegex.FindSubmatch(dump)
	if matches == nil {
		return fmt.Errorf("sandbox image could not be found in containerd config (version %d format)", configVersion)
	}
	sandboxImage := string(matches[1])
	zap.L().Info("Found sandbox image", zap.String("image", sandboxImage))

	zap.L().Info("Fetching ECR authorization token...")
	ecrUserToken, err := ecr.GetAuthorizationToken(awsConfig)
	if err != nil {
		return err
	}

	client, err := remote.NewImageService(ContainerRuntimeEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	imageSpec := &v1.ImageSpec{Image: sandboxImage}
	authConfig := &v1.AuthConfig{Auth: ecrUserToken}

	return util.RetryExponentialBackoff(3, 2*time.Second, func() error {
		zap.L().Info("Pulling sandbox image...", zap.String("image", sandboxImage))
		imageRef, err := client.PullImage(imageSpec, authConfig, nil)
		if err != nil {
			return err
		}
		zap.L().Info("Finished pulling sandbox image", zap.String("image-ref", imageRef))
		return nil
	})
}

// parseConfigVersion extracts the config version from containerd config dump output
func parseConfigVersion(dump []byte) (int, error) {
	matches := containerdConfigVersionRegex.FindSubmatch(dump)
	if matches == nil {
		return 0, fmt.Errorf("config version not found in containerd config dump")
	}

	versionStr := string(matches[1])
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse config version '%s': %w", versionStr, err)
	}

	return version, nil
}
