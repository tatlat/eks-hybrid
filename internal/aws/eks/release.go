package eks

import (
	"context"
	"crypto/sha256"
	"fmt"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"sigs.k8s.io/yaml"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/util"
)

// set build time
var manifestUrl string

type Manifest struct {
	SupportedReleases []SupportedRelease `json:"supported_releases"`
}

type SupportedRelease struct {
	MajorMinorVersion  string         `json:"major_minor_version"`
	LatestPatchVersion string         `json:"latest_patch_version"`
	PatchReleases      []PatchRelease `json:"patch_releases"`
}

type PatchRelease struct {
	Version      string     `json:"version"`
	PatchVersion string     `json:"patch_version"`
	ReleaseDate  string     `json:"release_date"`
	Artifacts    []Artifact `json:"artifacts"`
}

type Artifact struct {
	Name        string `json:"name"`
	Arch        string `json:"arch"`
	OS          string `json:"os"`
	URI         string `json:"uri"`
	ChecksumURI string `json:"checksum_uri"`
}

// FindLatestRelease finds the latest release based on version. Version should be a semantic version
// (without a 'v' prepended). For example, 1.29.
func FindLatestRelease(ctx context.Context, version string) (PatchRelease, error) {
	if version == "" {
		return PatchRelease{}, fmt.Errorf("version is empty")
	}

	if !semver.IsValid("v" + version) {
		return PatchRelease{}, fmt.Errorf("invalid semantic version: %v", version)
	}

	// Read in the release manifest
	manifest, err := getReleaseManifest(ctx)
	if err != nil {
		return PatchRelease{}, err
	}

	// Check if input is major.minor or major.minor.patch
	// semver.MajorMinor requires "v" to prepended
	majorMinorVersion := semver.MajorMinor("v" + version)

	// remove v from the majorMinor
	majorMinorVersion = strings.ReplaceAll(majorMinorVersion, "v", "")

	// find if input version has patch version
	patch, hasPatchVersion := strings.CutPrefix(version, majorMinorVersion+".")

	// Find the patch release
	for _, supportedRelease := range manifest.SupportedReleases {
		if supportedRelease.MajorMinorVersion == majorMinorVersion {
			for _, release := range supportedRelease.PatchReleases {
				// Check if patch version was provided in the input
				if hasPatchVersion {
					if release.PatchVersion == patch {
						return release, nil
					}
				} else {
					if release.PatchVersion == supportedRelease.LatestPatchVersion {
						return release, nil
					}
				}
			}
		}
	}

	// If patch version was provided in the input and associated release was not found, throw an error
	if hasPatchVersion {
		return PatchRelease{}, fmt.Errorf("input semver did not match with available releases. Try again with major.minor version")
	}

	return PatchRelease{}, fmt.Errorf("input semver did not match with any available releases")
}

// Read from the manifest file on s3 and parse into Manifest struct
func getReleaseManifest(ctx context.Context) (*Manifest, error) {
	yamlFileData, err := util.GetHttpFile(ctx, manifestUrl)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	err = yaml.Unmarshal(yamlFileData, &manifest)
	if err != nil {
		return nil, errors.Wrap(err, "invalid yaml data in release manifest")
	}
	return &manifest, nil
}

// GetKubelet satisfies kubelet.Source.
func (r PatchRelease) GetKubelet(ctx context.Context) (artifact.Source, error) {
	return r.getSource(ctx, "kubelet")
}

// GetKubectl satisfies kubectl.Source.
func (r PatchRelease) GetKubectl(ctx context.Context) (artifact.Source, error) {
	return r.getSource(ctx, "kubectl")
}

// GetIAMAuthenticator satisfies iamrolesanywhere.IAMAuthenticatorSource.
func (r PatchRelease) GetIAMAuthenticator(ctx context.Context) (artifact.Source, error) {
	return r.getSource(ctx, "aws-iam-authenticator")
}

// GetImageCredentialProvider satisfies imagecredentialprovider.Source.
func (r PatchRelease) GetImageCredentialProvider(ctx context.Context) (artifact.Source, error) {
	return r.getSource(ctx, "ecr-credential-provider")
}

func (r PatchRelease) GetCniPlugins(ctx context.Context) (artifact.Source, error) {
	return r.getSource(ctx, "cni-plugins")
}

func (r PatchRelease) getSource(ctx context.Context, artifactName string) (artifact.Source, error) {
	for _, releaseArtifact := range r.Artifacts {
		if releaseArtifact.Name == artifactName && releaseArtifact.Arch == runtime.GOARCH && releaseArtifact.OS == runtime.GOOS {
			obj, err := util.GetHttpFileReader(ctx, releaseArtifact.URI)
			if err != nil {
				obj.Close()
				return nil, err
			}

			artifactChecksum, err := util.GetHttpFile(ctx, releaseArtifact.ChecksumURI)
			if err != nil {
				obj.Close()
				return nil, err
			}
			source, err := artifact.WithChecksum(obj, sha256.New(), artifactChecksum)
			if err != nil {
				return nil, err
			}
			return source, nil
		}
	}
	return nil, fmt.Errorf("could not find artifact for %s arch and %s os", runtime.GOARCH, runtime.GOOS)
}
