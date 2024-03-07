package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
)

func ShowReleaseArtifacts(ctx context.Context, r Release) error {
	ls, err := r.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(fmt.Sprintf("%v/%v", r.Version, r.ReleaseDate)),
	}, func(o *s3.Options) {
		// TODO(chrisdoherty) Investigate alternatives that optimize for geographical location.
		// Buckets aren't replicated so we need to use the right region for querying S3.
		o.Region = "us-west-2"
	})

	if err != nil {
		return err
	}

	for _, item := range ls.Contents {
		fmt.Println(*item.Key)
	}

	return nil
}
