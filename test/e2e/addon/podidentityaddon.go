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
	kubernetes *clientgo.Clientset
	iamClient  *iam.Client
	roleArn    string
}

const (
	podIdentityServiceAccount = "pod-identity-sa"
	namespace                 = "default"
)

func NewPodIdentityAddon(cluster, name string, k8sClient *clientgo.Clientset, iamClient *iam.Client, roleArn string) PodIdentityAddon {
	return PodIdentityAddon{
		Addon: Addon{
			Cluster:       cluster,
			Name:          name,
			Configuration: "{\"daemonsets\":{\"hybrid\":{\"create\": true}}}",
		},
		kubernetes: k8sClient,
		iamClient:  iamClient,
		roleArn:    roleArn,
	}
}

func (p PodIdentityAddon) Create(ctx context.Context, client *eks.Client, logger logr.Logger) error {
	if err := p.Addon.Create(ctx, client, logger); err != nil {
		return err
	}

	// Provision PodIdentity addon related resources
	// Create service account in kubernetes
	if err := kubernetes.NewServiceAccount(ctx, logger, p.kubernetes, namespace, podIdentityServiceAccount); err != nil {
		return err
	}

	createPodIdentityAssociationInput := &eks.CreatePodIdentityAssociationInput{
		ClusterName:    &p.Cluster,
		Namespace:      aws.String(namespace),
		RoleArn:        &p.roleArn,
		ServiceAccount: aws.String(podIdentityServiceAccount),
	}

	_, err := client.CreatePodIdentityAssociation(ctx, createPodIdentityAssociationInput)
	if err != nil && !errors.IsType(err, &types.ResourceInUseException{}) {
		return err
	}

	return nil
}
