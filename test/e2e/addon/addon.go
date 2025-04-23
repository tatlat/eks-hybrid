package addon

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/eks-hybrid/test/e2e/errors"
	kube "github.com/aws/eks-hybrid/test/e2e/kubernetes"
)

type Addon struct {
	Name          string
	Namespace     string
	Cluster       string
	Configuration string
}

const (
	backoff = 10 * time.Second
)

func (a Addon) Create(ctx context.Context, client *eks.Client, logger logr.Logger) error {
	logger.Info("Create cluster add-on", "ClusterAddon", a.Name)

	params := &eks.CreateAddonInput{
		ClusterName:         &a.Cluster,
		AddonName:           &a.Name,
		ConfigurationValues: &a.Configuration,
	}

	_, err := client.CreateAddon(ctx, params)

	if err == nil || errors.IsType(err, &types.ResourceInUseException{}) {
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

	for {
		addon, err := a.describe(ctx, client)
		if err != nil {
			logger.Info("Failed to describe cluster add-on", "Error", err)
		} else {
			if addon.Status == types.AddonStatusActive {
				return nil
			}

			if addon.Status == types.AddonStatusCreateFailed ||
				addon.Status == types.AddonStatusDegraded ||
				addon.Status == types.AddonStatusDeleteFailed ||
				addon.Status == types.AddonStatusUpdateFailed {
				return fmt.Errorf("add-on %s is in errored terminal status: %s", a.Name, addon.Status)
			}
		}

		logger.Info("Wait for add-on to be ACTIVE", "ClusterAddon", a.Name)

		select {
		case <-ctx.Done():
			return fmt.Errorf("add-on %s still has status %s: %w", a.Name, addon.Status, ctx.Err())
		case <-time.After(backoff):
		}
	}
}

func (a Addon) CreateAddon(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	if err := a.Create(ctx, eksClient, logger); err != nil {
		return err
	}

	if err := a.WaitUntilActive(ctx, eksClient, logger); err != nil {
		return err
	}

	addonListOptions := getAddonListOptions(a.Name)
	if err := kube.WaitForPodsToBeRunning(ctx, k8s, addonListOptions, a.Namespace, logger); err != nil {
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

	if err == nil || errors.IsType(err, &types.ResourceNotFoundException{}) {
		// Ignore if add-on doesn't exist
		return nil
	}
	return err
}

func getPodLogOptions(containerName string, lines *int64) *corev1.PodLogOptions {
	return &corev1.PodLogOptions{
		Container: containerName, // specify container name if multiple containers
		TailLines: lines,         // get last N lines
	}
}

func getAddonListOptions(addonName string) v1.ListOptions {
	return v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "app.kubernetes.io/name", addonName),
	}
}
