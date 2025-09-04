package addon

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	e2errors "github.com/aws/eks-hybrid/test/e2e/errors"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	peeredtypes "github.com/aws/eks-hybrid/test/e2e/peered/types"
)

const (
	s3CSIDriverName      = "aws-mountpoint-s3-csi-driver"
	s3CSIDriverNamespace = "kube-system"
	s3CSIControllerName  = "s3-csi-controller"
	s3CSINodeName        = "s3-csi-node"
	s3CSIDriverSAName    = "s3-csi-driver-sa"
	s3CSIWaitTimeout     = 5 * time.Minute
)

// S3MountpointCSIDriverTest tests the S3 mountpoint CSI driver addon
type S3MountpointCSIDriverTest struct {
	Cluster            string
	addon              *Addon
	K8S                peeredtypes.K8s
	EKSClient          *eks.Client
	K8SConfig          *rest.Config
	Logger             logr.Logger
	PodIdentityRoleArn string
}

// Create installs the S3 mountpoint CSI driver addon
func (s *S3MountpointCSIDriverTest) Create(ctx context.Context) error {
	s.addon = &Addon{
		Cluster:   s.Cluster,
		Namespace: s3CSIDriverNamespace,
		Name:      s3CSIDriverName,
		Version:   "v2.0.0-eksbuild.1", // need to specify v2 version explicitly for now since v1 is the default version to install
	}

	// Create pod identity association for the addon's service account
	if err := s.setupPodIdentity(ctx); err != nil {
		return fmt.Errorf("failed to setup Pod Identity for S3 CSI driver: %v", err)
	}

	if err := s.addon.CreateAndWaitForActive(ctx, s.EKSClient, s.K8S, s.Logger); err != nil {
		return err
	}

	// TODO: remove the following call once the addon is updated to work with hybrid nodes
	// Remove anti affinity to allow s3-csi-node to be deployed to hybrid nodes
	if err := kubernetes.RemoveDaemonSetAntiAffinity(ctx, s.Logger, s.K8S, s3CSIDriverNamespace, s3CSINodeName); err != nil {
		return fmt.Errorf("failed to remove anti affinity: %w", err)
	}

	// Wait for CSI driver controller deployment and node daemonset to be ready
	if err := kubernetes.DeploymentWaitForReplicas(ctx, s3CSIWaitTimeout, s.K8S, s3CSIDriverNamespace, s3CSIControllerName); err != nil {
		return fmt.Errorf("controller deployment %s not ready: %w", s3CSIControllerName, err)
	}

	if err := kubernetes.DaemonSetWaitForReady(ctx, s.Logger, s.K8S, s3CSIDriverNamespace, s3CSINodeName); err != nil {
		return fmt.Errorf("node daemonset %s not ready: %w", s3CSINodeName, err)
	}

	return nil
}

// Validate checks if S3 mountpoint CSI driver is working correctly
func (s *S3MountpointCSIDriverTest) Validate(ctx context.Context) error {
	// TODO: add validate later
	return nil
}

func (s *S3MountpointCSIDriverTest) Delete(ctx context.Context) error {
	return s.addon.Delete(ctx, s.EKSClient, s.Logger)
}

func (s *S3MountpointCSIDriverTest) setupPodIdentity(ctx context.Context) error {
	s.Logger.Info("Setting up Pod Identity for S3 CSI driver")

	// Create Pod Identity Association for the addon's service account
	createAssociationInput := &eks.CreatePodIdentityAssociationInput{
		ClusterName:    aws.String(s.Cluster),
		Namespace:      aws.String(s3CSIDriverNamespace),
		RoleArn:        aws.String(s.PodIdentityRoleArn),
		ServiceAccount: aws.String(s3CSIDriverSAName),
	}

	createAssociationOutput, err := s.EKSClient.CreatePodIdentityAssociation(ctx, createAssociationInput)

	if err != nil && e2errors.IsType(err, &ekstypes.ResourceInUseException{}) {
		s.Logger.Info("Pod Identity Association already exists for service account: %s", awsPcaServiceAccountName)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to create Pod Identity Association: %v", err)
	}

	s.Logger.Info("Created Pod Identity Association", "associationID", *createAssociationOutput.Association.AssociationId)
	return nil
}
