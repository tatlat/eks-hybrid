package eks

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"golang.org/x/mod/semver"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

// bucket is the EKS bucket housing officially released EKS artifacts.
const bucket = "amazon-eks"

// S3ObjectReader provides read only APIs for S3 interacftion. It is intended to be implemented
// by the official AWS Go SDK.
type S3ObjectReader interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// Release is an official EKS release. It provides methods for retrieving release artifacts.
type Release struct {
	Version     string
	ReleaseDate string
	Client      S3ObjectReader
}

// FindLatestRelease finds the latest release based on version. Version should be a semantic version
// (without a 'v' prepended). For example, 1.29. The client is re-used for future fetch operations.
func FindLatestRelease(ctx context.Context, client S3ObjectReader, version string) (Release, error) {
	if version == "" {
		return Release{}, errors.New("version is empty")
	}

	if !semver.IsValid("v" + version) {
		return Release{}, fmt.Errorf("invalid semantic version: %v", version)
	}

	ls, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(version),
	}, func(o *s3.Options) {
		// TODO(chrisdoherty) Investigate alternatives that optimize for geographical location.
		// Buckets aren't replicated so we need to use the right region for querying S3.
		o.Region = "us-west-2"
	})
	if err != nil {
		return Release{}, err
	}

	latestVersion := "0.0.0"
	var releaseDate string
	for _, v := range ls.Contents {
		// Expected v.Key format: 1.27.1/2023-04-19/.*
		keyParts := strings.Split(*v.Key, "/")

		if len(keyParts) < 2 {
			return Release{}, fmt.Errorf("unexpected response when listing versions: %v", *v.Key)
		}

		if !semver.IsValid("v" + keyParts[0]) {
			return Release{}, fmt.Errorf("unexpected value for kubernetes version: %v", keyParts[0])
		}

		if semver.Compare("v"+latestVersion, "v"+keyParts[0]) < 0 {
			latestVersion = keyParts[0]
			releaseDate = keyParts[1]
		}
	}

	return Release{
		Version:     latestVersion,
		ReleaseDate: releaseDate,
		Client:      client,
	}, nil
}

// GetKubelet satisfies kubelet.Source.
func (r Release) GetKubelet(ctx context.Context) (artifact.Source, error) {
	obj, err := r.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(r.getKey("kubelet")),
	}, func(o *s3.Options) {
		// TODO(chrisdoherty) Investigate alternatives that optimize for geographical location.
		// Buckets aren't replicated so we need to use the right region for querying S3.
		o.Region = "us-west-2"
	})
	if err != nil {
		return artifact.Source{}, err
	}

	return artifact.Source{ReadCloser: obj.Body}, nil
}

// GetKubectl satisfies kubectl.Source.
func (r Release) GetKubectl(ctx context.Context) (artifact.Source, error) {
	obj, err := r.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(r.getKey("kubectl")),
	}, func(o *s3.Options) {
		// TODO(chrisdoherty) Investigate alternatives that optimize for geographical location.
		// Buckets aren't replicated so we need to use the right region for querying S3.
		o.Region = "us-west-2"
	})
	if err != nil {
		return artifact.Source{}, err
	}

	return artifact.Source{ReadCloser: obj.Body}, nil
}

// GetIAMAuthenticator satisfies iamrolesanywhere.IAMAuthenticatorSource.
func (r Release) GetIAMAuthenticator(ctx context.Context) (artifact.Source, error) {
	obj, err := r.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(r.getKey("aws-iam-authenticator")),
	}, func(o *s3.Options) {
		// TODO(chrisdoherty) Investigate alternatives that optimize for geographical location.
		// Buckets aren't replicated so we need to use the right region for querying S3.
		o.Region = "us-west-2"
	})
	if err != nil {
		return artifact.Source{}, err
	}

	return artifact.Source{ReadCloser: obj.Body}, nil
}

// GetImageCredentialProvider satisfies imagecredentialprovider.Source.
func (r Release) GetImageCredentialProvider(ctx context.Context) (artifact.Source, error) {
	obj, err := r.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(r.getKey("ecr-credental-provider")),
	}, func(o *s3.Options) {
		// TODO(chrisdoherty) Investigate alternatives that optimize for geographical location.
		// Buckets aren't replicated so we need to use the right region for querying S3.
		o.Region = "us-west-2"
	})
	if err != nil {
		return artifact.Source{}, err
	}

	return artifact.Source{ReadCloser: obj.Body}, nil
}

func (r Release) getKey(artifact string) string {
	return fmt.Sprintf("%v/%v/linux/%v/%v", r.Version, r.ReleaseDate, runtime.GOARCH, artifact)
}
