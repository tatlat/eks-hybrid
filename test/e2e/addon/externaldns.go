package addon

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	peeredtypes "github.com/aws/eks-hybrid/test/e2e/peered/types"
)

const (
	externalDNSName           = "external-dns"
	externalDNSNamespace      = "external-dns"
	externalDNSDeploymentName = "external-dns"
	externalDNSWaitTimeout    = 5 * time.Minute
)

// ExternalDNSTest tests the external-dns addon
type ExternalDNSTest struct {
	Cluster   string
	addon     *Addon
	K8S       peeredtypes.K8s
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

// Create installs the external-dns addon
func (e *ExternalDNSTest) Create(ctx context.Context) error {
	e.addon = &Addon{
		Cluster:   e.Cluster,
		Namespace: externalDNSNamespace,
		Name:      externalDNSName,
	}

	if err := e.addon.CreateAndWaitForActive(ctx, e.EKSClient, e.K8S, e.Logger); err != nil {
		return err
	}

	// TODO: remove the following call once the addon is updated to work with hybrid nodes
	// Remove anti affinity to allow external-dns to be deployed to hybrid nodes
	if err := kubernetes.RemoveDeploymentAntiAffinity(ctx, e.K8S, externalDNSDeploymentName, externalDNSNamespace, e.Logger); err != nil {
		return fmt.Errorf("failed to remove anti affinity: %w", err)
	}

	// Wait for external-dns deployment to be ready
	if err := kubernetes.DeploymentWaitForReplicas(ctx, externalDNSWaitTimeout, e.K8S, externalDNSNamespace, externalDNSDeploymentName); err != nil {
		return fmt.Errorf("deployment %s not ready: %w", externalDNSDeploymentName, err)
	}

	return nil
}

// Validate checks if external-dns is working correctly
func (e *ExternalDNSTest) Validate(ctx context.Context) error {
	// TODO: add validate later
	return nil
}

func (e *ExternalDNSTest) Delete(ctx context.Context) error {
	return e.addon.Delete(ctx, e.EKSClient, e.Logger)
}
