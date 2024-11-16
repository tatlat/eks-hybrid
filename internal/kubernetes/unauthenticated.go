package kubernetes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/validation"
)

func MakeUnauthenticatedRequest(ctx context.Context, endpoint string, caCertificate []byte) error {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertificate) {
		return validation.WithRemediation(errors.New("failed to parse Cluster CA certificate"),
			"Ensure the Cluster CA certificate provided is correct.",
		)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return validation.WithRemediation(err, "Ensure the Kubernetes API server endpoint provided is correct.")
	}

	resp, err := client.Do(req)
	if err != nil {
		return validation.WithRemediation(err, "Ensure the provided Kubernetes API server endpoint is correct and the CA certificate is valid for that endpoint.")
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading unauthenticated request response body: %w", err)
	}

	apiServerResp := &apiServerResponse{}
	if err = json.Unmarshal(body, apiServerResp); err != nil {
		return fmt.Errorf("unmarshalling unauthenticated request response: %w", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		return validation.WithRemediation(fmt.Errorf("expected status code from unauthenticated request %d, got %d. Message: %s", http.StatusForbidden, resp.StatusCode, apiServerResp.Message),
			"Ensure the Kubernetes API server endpoint provided is correct and the CA certificate is valid for that endpoint.",
		)
	}

	return nil
}

type apiServerResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code    int64  `json:"code"`
}

func CheckUnauthenticatedAccess(ctx context.Context, informer validation.Informer, node *api.NodeConfig) error {
	name := "kubernetes-unauthenticated-request"
	var err error
	informer.Starting(ctx, name, "Validating unauthenticated request to Kubernetes API endpoint")
	defer func() {
		informer.Done(ctx, name, err)
	}()

	if err = MakeUnauthenticatedRequest(ctx, node.Spec.Cluster.APIServerEndpoint, node.Spec.Cluster.CertificateAuthority); err != nil {
		return err
	}

	return nil
}
