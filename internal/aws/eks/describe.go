package eks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	eks_sdk "github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go/aws"
	smithy "github.com/aws/smithy-go"
	smithydocument "github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var NewClient = eks_sdk.NewFromConfig

// DescribeCluster call the DescribeCluster EKS API. It's equivalent to the method provided
// by the Go SDK except it's also able to read the new field remoteNetworkConfig. Once this new
// field is public and part of the SDK, we can update the SDK module and delete this code.
func DescribeCluster(ctx context.Context, c *eks_sdk.Client, name string) (*DescribeClusterOutput, error) {
	describeResponse := &DescribeClusterOutput{}
	in := &eks_sdk.DescribeClusterInput{
		Name: aws.String(name),
	}
	_, err := c.DescribeCluster(ctx, in, func(o *eks_sdk.Options) {
	}, eks_sdk.WithAPIOptions(func(s *middleware.Stack) error {
		return s.Deserialize.Add(middleware.DeserializeMiddlewareFunc("CustomDescribeCluster", func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
			out, metadata, err := next.HandleDeserialize(ctx, in)
			if err != nil {
				return out, metadata, err
			}

			response, ok := out.RawResponse.(*smithyhttp.Response)
			if !ok {
				return out, metadata, &smithy.DeserializationError{Err: fmt.Errorf("unknown transport type %T", out.RawResponse)}
			}

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				return out, metadata, nil
			}

			var buffer bytes.Buffer
			if _, err := io.Copy(&buffer, response.Body); err != nil {
				return out, metadata, &smithy.DeserializationError{Err: fmt.Errorf("failed to read DescribeCluster response: %w", err)}
			}

			if err := json.Unmarshal(buffer.Bytes(), &describeResponse); err != nil {
				return out, metadata, &smithy.DeserializationError{Err: fmt.Errorf("failed to unmarshal DescribeCluster response: %w", err)}
			}

			return out, metadata, nil
		}), middleware.After)
	}))
	if err != nil {
		return nil, err
	}

	return describeResponse, nil
}

type DescribeClusterOutput struct {
	smithydocument.NoSerde

	// The full description of your specified cluster.
	Cluster *Cluster `locationName:"cluster" type:"structure"`
}
