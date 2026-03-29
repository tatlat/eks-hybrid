package addon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"

	e2eerrors "github.com/aws/eks-hybrid/test/e2e/errors"
)

// ErrAddonNotAvailable is returned when an addon is not available in the current region/partition.
var ErrAddonNotAvailable = errors.New("addon not available in this region")

type Addon struct {
	Name                    string
	Namespace               string
	Cluster                 string
	Configuration           string
	Version                 string
	PodIdentityAssociations []PodIdentityAssociation
}

type PodIdentityAssociation struct {
	RoleArn        string
	ServiceAccount string
}

const (
	addonPollInterval = 10 * time.Second
	addonPollTimeout  = 5 * time.Minute
	defaultNamespace  = "default"
)

// IsAvailable returns true if the addon has at least one version available via DescribeAddonVersions.
// This is used to skip tests in regions/partitions where the addon is not offered.
func (a Addon) IsAvailable(ctx context.Context, client *eks.Client) (bool, error) {
	out, err := client.DescribeAddonVersions(ctx, &eks.DescribeAddonVersionsInput{
		AddonName: &a.Name,
	})
	if err != nil {
		// ResourceNotFoundException means the addon name is unknown in this partition.
		if e2eerrors.IsType(err, &types.ResourceNotFoundException{}) {
			return false, nil
		}
		return false, fmt.Errorf("describing addon versions for %s: %w", a.Name, err)
	}
	return len(out.Addons) > 0, nil
}

func (a Addon) Create(ctx context.Context, client *eks.Client, logger logr.Logger) error {
	logger.Info("Create cluster add-on", "ClusterAddon", a.Name)

	var namespaceConfig *types.AddonNamespaceConfigRequest
	if a.Namespace != "" {
		namespaceConfig = &types.AddonNamespaceConfigRequest{
			Namespace: &a.Namespace,
		}
	}

	var podIdentityAssociations []types.AddonPodIdentityAssociations
	for _, association := range a.PodIdentityAssociations {
		podIdentityAssociations = append(podIdentityAssociations, types.AddonPodIdentityAssociations{
			RoleArn:        &association.RoleArn,
			ServiceAccount: &association.ServiceAccount,
		})
	}

	params := &eks.CreateAddonInput{
		ClusterName:             &a.Cluster,
		AddonName:               &a.Name,
		ConfigurationValues:     &a.Configuration,
		AddonVersion:            &a.Version,
		NamespaceConfig:         namespaceConfig,
		PodIdentityAssociations: podIdentityAssociations,
	}

	_, err := client.CreateAddon(ctx, params)

	if err == nil || e2eerrors.IsType(err, &types.ResourceInUseException{}) {
		// Ignore if add-on is already created
		return nil
	}
	return err
}

func (a Addon) describe(ctx context.Context, client *eks.Client) (*types.Addon, error) {
	params := &eks.DescribeAddonInput{
		ClusterName: &a.Cluster,
		AddonName:   &a.Name,
	}

	describeAddonOutput, err := client.DescribeAddon(ctx, params)
	if err != nil {
		return nil, err
	}

	return describeAddonOutput.Addon, nil
}

func (a Addon) WaitUntilActive(ctx context.Context, client *eks.Client, logger logr.Logger) error {
	logger.Info("Describe cluster add-on", "ClusterAddon", a.Name)

	err := wait.PollUntilContextTimeout(ctx, addonPollInterval, addonPollTimeout, true, func(ctx context.Context) (bool, error) {
		addon, err := a.describe(ctx, client)
		if err != nil {
			logger.Error(err, "Failed to describe cluster add-on")
			return false, nil
		}

		if addon.Status == types.AddonStatusCreateFailed ||
			addon.Status == types.AddonStatusDeleteFailed ||
			addon.Status == types.AddonStatusUpdateFailed {
			return false, fmt.Errorf("add-on %s is in errored terminal status: %s", a.Name, addon.Status)
		}

		if addon.Status == types.AddonStatusActive || addon.Status == types.AddonStatusDegraded {
			// Add-on is either active or degraded
			// in our case degraded is acceptable since this is usually due to there not being enough replicas
			// which happens as we create and delete nodes
			return true, nil
		}

		logger.Info("Waiting for add-on to be ACTIVE", "ClusterAddon", a.Name)
		return false, nil
	})

	return err
}

func (a Addon) CreateAndWaitForActive(ctx context.Context, eksClient *eks.Client, k8s clientgo.Interface, logger logr.Logger) error {
	available, err := a.IsAvailable(ctx, eksClient)
	if err != nil {
		return err
	}
	if !available {
		logger.Info("Addon not available in this region, skipping", "addon", a.Name)
		return fmt.Errorf("%w: %s", ErrAddonNotAvailable, a.Name)
	}

	if err := a.Create(ctx, eksClient, logger); err != nil {
		return err
	}

	if err := a.WaitUntilActive(ctx, eksClient, logger); err != nil {
		return err
	}

	return nil
}

func (a Addon) Delete(ctx context.Context, client *eks.Client, logger logr.Logger) error {
	logger.Info("Delete cluster add-on", "ClusterAddon", a.Name)

	params := &eks.DeleteAddonInput{
		ClusterName: &a.Cluster,
		AddonName:   &a.Name,
	}

	_, err := client.DeleteAddon(ctx, params)

	// Add-on is deleted already
	if e2eerrors.IsType(err, &types.ResourceNotFoundException{}) {
		return nil
	}

	// Otherwise let's poll until it's deleted
	err = wait.PollUntilContextTimeout(ctx, addonPollInterval, addonPollTimeout, true, func(ctx context.Context) (bool, error) {
		_, err := a.describe(ctx, client)
		if e2eerrors.IsType(err, &types.ResourceNotFoundException{}) {
			return true, nil
		}

		logger.Info("Waiting for add-on to be deleted", "ClusterAddon", a.Name)
		return false, nil
	})

	return err
}
