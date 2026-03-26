package aws

import (
	"context"
	"os"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"github.com/aws/eks-hybrid/internal/util"
)

// set build time
var manifestUrl string

// getManifestURL returns the appropriate manifest URL based on the region/partition
// If the region is in aws-cn partition, it uses a China-specific URL
// Otherwise, it defaults to the embedded manifestUrl
func getManifestURL(region string) string {
	if region == "" {
		// No region provided, use default embedded URL
		return manifestUrl
	}

	// Detect partition from region
	partition := GetPartitionFromRegionFallback(region)

	// For aws-cn partition, use China-specific manifest host
	if partition == "aws-cn" {
		return "https://eks-hybrid-assets.awsstatic.cn/manifest.yaml"
	}

	// For all other partitions, use the embedded default URL
	return manifestUrl
}

type Manifest struct {
	SupportedEksReleases     []SupportedEksRelease     `json:"supported_eks_releases"`
	IamRolesAnywhereReleases []IamRolesAnywhereRelease `json:"iam_roles_anywhere_releases"`
	SsmReleases              []SsmRelease              `json:"ssm_releases"`
	RegionConfig             RegionConfig              `json:"region_config"`
}

type SupportedEksRelease struct {
	MajorMinorVersion  string            `json:"major_minor_version"`
	LatestPatchVersion string            `json:"latest_patch_version"`
	PatchReleases      []EksPatchRelease `json:"patch_releases"`
}

type EksPatchRelease struct {
	Version      string     `json:"version"`
	PatchVersion string     `json:"patch_version"`
	ReleaseDate  string     `json:"release_date"`
	Artifacts    []Artifact `json:"artifacts"`
}

type IamRolesAnywhereRelease struct {
	Version   string     `json:"version"`
	Artifacts []Artifact `json:"artifacts"`
}

type SsmRelease struct {
	Version   string     `json:"version"`
	Artifacts []Artifact `json:"artifacts"`
}

// RegionConfig represents the structure of the manifest file
type RegionConfig map[string]RegionData

// RegionData represents data for a specific region
type RegionData struct {
	EcrAccountID  string          `json:"ecr_account_id"`
	Partition     string          `json:"partition"`
	DnsSuffix     string          `json:"dns_suffix"`
	CredProviders map[string]bool `json:"cred_providers"`
}

type Artifact struct {
	Name        string `json:"name"`
	Arch        string `json:"arch"`
	OS          string `json:"os"`
	URI         string `json:"uri"`
	ChecksumURI string `json:"checksum_uri,omitempty"`
	GzipURI     string `json:"gzip_uri,omitempty"`
}

// Read from the manifest file on s3 and parse into Manifest struct
// region is used to determine the appropriate manifest URL for different partitions (e.g., aws-cn)
func getReleaseManifest(ctx context.Context, region string) (*Manifest, error) {
	manifestURL := getManifestURL(region)
	yamlFileData, err := util.GetHttpFile(ctx, manifestURL)
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

// getReleaseManifestFromURI reads from a URI (file:// or https://) and parses into Manifest struct
func getReleaseManifestFromURI(ctx context.Context, manifestURI string) (*Manifest, error) {
	var yamlFileData []byte
	var err error

	// Check if the URI uses file:// protocol
	if strings.HasPrefix(manifestURI, "file://") {
		// Strip file:// prefix and read from local file
		filePath := strings.TrimPrefix(manifestURI, "file://")
		yamlFileData, err = os.ReadFile(filePath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading manifest file from file:// URI: %s", manifestURI)
		}
	} else if strings.HasPrefix(manifestURI, "https://") {
		// Download from HTTPS URL
		yamlFileData, err = util.GetHttpFile(ctx, manifestURI)
		if err != nil {
			return nil, errors.Wrapf(err, "downloading manifest file from https:// URI: %s", manifestURI)
		}
	} else {
		// For backward compatibility, treat as a plain file path
		yamlFileData, err = os.ReadFile(manifestURI)
		if err != nil {
			return nil, errors.Wrapf(err, "reading manifest file: %s (hint: use file:// or https:// prefix)", manifestURI)
		}
	}

	var manifest Manifest
	err = yaml.Unmarshal(yamlFileData, &manifest)
	if err != nil {
		return nil, errors.Wrap(err, "invalid yaml data in release manifest")
	}
	return &manifest, nil
}
