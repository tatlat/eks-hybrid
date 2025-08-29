package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	ik8s "github.com/aws/eks-hybrid/internal/kubernetes"
)

const (
	daemonSetWaitTimeout = 3 * time.Minute
)

// GetDaemonSet returns a daemonset by name in a specific namespace
// It will wait for the daemonset to exist up to 3 minutes
func GetDaemonSet(ctx context.Context, logger logr.Logger, k8s kubernetes.Interface, namespace, name string) (*appsv1.DaemonSet, error) {
	ds, err := ik8s.GetAndWait(ctx, daemonSetWaitTimeout, k8s.AppsV1().DaemonSets(namespace), name, func(ds *appsv1.DaemonSet) bool {
		// Return true to stop polling as soon as we get the daemonset
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("waiting daemonset %s in namespace %s: %w", name, namespace, err)
	}
	return ds, nil
}

func DaemonSetWaitForReady(ctx context.Context, logger logr.Logger, k8s kubernetes.Interface, namespace, name string) error {
	if _, err := ik8s.GetAndWait(ctx, daemonSetWaitTimeout, k8s.AppsV1().DaemonSets(namespace), name, func(ds *appsv1.DaemonSet) bool {
		return ds.Status.NumberReady == ds.Status.DesiredNumberScheduled
	}); err != nil {
		return fmt.Errorf("daemonset %s replicas never became ready: %v", name, err)
	}
	return nil
}

// RemoveDaemonSetAntiAffinity removes node affinity rules from a daemonset that would prevent pods from being scheduled on hybrid nodes.
// This is useful to test EKS add-on before anti-affinity rule for hybrid nodes is removed.
// Once anti-affinity rule is removed, then caller no longer needs to call this method.
func RemoveDaemonSetAntiAffinity(ctx context.Context, logger logr.Logger, k8s kubernetes.Interface, namespace, name string) error {
	logger.Info("Removing node affinity rules to allow scheduling on hybrid nodes", "daemonset", name, "namespace", namespace)

	// Get the current daemonset to check if it has affinity rules
	daemonset, err := k8s.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting daemonset %s in namespace %s: %w", name, namespace, err)
	}

	if daemonset.Spec.Template.Spec.Affinity == nil {
		logger.Info("DaemonSet has no affinity rules, nothing to remove", "daemonset", name)
		return nil
	}

	if daemonset.Spec.Template.Spec.Affinity.NodeAffinity == nil {
		logger.Info("DaemonSet has no node affinity rules", "daemonset", name)
		return nil
	}

	// Create a JSON patch to remove the nodeAffinity field
	// For now we remove all nodeAffinity rules, which should be okay for our e2e tests
	patchJSON := []map[string]interface{}{
		{
			"op":   "remove",
			"path": "/spec/template/spec/affinity/nodeAffinity",
		},
	}

	patchBytes, err := json.Marshal(patchJSON)
	if err != nil {
		return fmt.Errorf("marshaling daemonset patch: %w", err)
	}

	// Apply the JSON patch
	_, err = k8s.AppsV1().DaemonSets(namespace).Patch(ctx, name, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patching daemonset %s to remove node affinity rules: %w", name, err)
	}

	logger.Info("Successfully removed node affinity rules from daemonset", "daemonset", name)
	return nil
}
