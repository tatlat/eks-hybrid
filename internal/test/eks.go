package test

import (
	"net/http"
	"testing"

	"github.com/aws/eks-hybrid/internal/aws/eks"
)

// NewEKSDescribeClusterAPI creates a new TestServer that behaves like the EKS DescribeCluster API.
func NewEKSDescribeClusterAPI(tb testing.TB, resp *eks.DescribeClusterOutput) TestServer {
	return NewHTTPSServerForJSON(tb, http.StatusOK, resp)
}
