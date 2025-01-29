package addon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/go-logr/logr"
	clientgo "k8s.io/client-go/kubernetes"

	"github.com/aws/eks-hybrid/test/e2e/errors"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
)

type PodIdentityAddon struct {
	Addon
	Kubernetes         *clientgo.Clientset
	IAMClient          *iam.Client
	PodIdentityRoleArn string
}

const (
	podIdentityServiceAccount = "pod-identity-sa"
	namespace                 = "default"
)

func (p PodIdentityAddon) Create(ctx context.Context, client *eks.Client, logger logr.Logger) error {
	if err := p.Addon.Create(ctx, client, logger); err != nil {
		return err
	}

	// Provision PodIdentity addon related resources
	// Create service account in kubernetes
	if err := kubernetes.NewServiceAccount(ctx, logger, p.Kubernetes, namespace, podIdentityServiceAccount); err != nil {
		return err
	}

	createPodIdentityAssociationInput := &eks.CreatePodIdentityAssociationInput{
		ClusterName:    &p.Cluster,
		Namespace:      aws.String(namespace),
		RoleArn:        &p.PodIdentityRoleArn,
		ServiceAccount: aws.String(podIdentityServiceAccount),
	}

	_, err := client.CreatePodIdentityAssociation(ctx, createPodIdentityAssociationInput)
	if err != nil && !errors.IsType(err, &types.ResourceInUseException{}) {
		return err
	}

	return nil
}
