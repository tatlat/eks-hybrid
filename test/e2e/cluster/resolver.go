package cluster

import (
	"context"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
)

type eksResolverV2 struct {
	endpoint string
}

// ResolveEndpoint resolves to a custom endpoint if not empty or default otherwise.
func (r *eksResolverV2) ResolveEndpoint(ctx context.Context, params eks.EndpointParameters) (
	smithyendpoints.Endpoint, error,
) {
	if r.endpoint != "" {
		u, err := url.Parse(r.endpoint)
		if err != nil {
			return smithyendpoints.Endpoint{}, err
		}
		return smithyendpoints.Endpoint{
			URI: *u,
		}, nil
	}

	// delegate back to the default v2 resolver otherwise
	return eks.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
}
