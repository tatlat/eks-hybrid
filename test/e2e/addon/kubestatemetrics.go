package addon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	kubeStateMetricsName      = "kube-state-metrics"
	kubeStateMetricsNamespace = "kube-state-metrics"
)

// KubeStateMetricsTest tests the kube-state-metrics addon
type KubeStateMetricsTest struct {
	Cluster   string
	addon     Addon
	K8S       clientgo.Interface
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

// Run executes the full test sequence
func (k *KubeStateMetricsTest) Run(ctx context.Context) error {
	k.addon = Addon{
		Cluster:   k.Cluster,
		Namespace: kubeStateMetricsNamespace,
		Name:      kubeStateMetricsName,
	}

	if err := k.Create(ctx); err != nil {
		return err
	}

	if err := k.Validate(ctx); err != nil {
		return err
	}

	return nil
}

// Create installs the kube-state-metrics addon
func (k *KubeStateMetricsTest) Create(ctx context.Context) error {
	if err := k.addon.CreateAddon(ctx, k.EKSClient, k.K8S, k.Logger); err != nil {
		return err
	}

	// Wait for deployment to be ready
	if err := kubernetes.WaitForDeploymentReady(ctx, k.Logger, k.K8S, k.addon.Namespace, k.addon.Name); err != nil {
		return err
	}

	return nil
}

// Validate checks if kube-state-metrics is providing metrics
func (k *KubeStateMetricsTest) Validate(ctx context.Context) error {
	k.Logger.Info("Checking if kube-state-metrics is providing metrics")

	// Find the service for kube-state-metrics
	svc, err := k.K8S.CoreV1().Services(kubeStateMetricsNamespace).Get(ctx, kubeStateMetricsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service: %v", err)
	}

	if len(svc.Spec.Ports) == 0 {
		return fmt.Errorf("no ports found for service")
	}

	// Find the metrics port (typically 8080)
	var metricsPort int32
	for _, port := range svc.Spec.Ports {
		if port.Name == "http" || port.Name == "metrics" {
			metricsPort = port.Port
			break
		}
	}

	if metricsPort == 0 {
		metricsPort = svc.Spec.Ports[0].Port // Fallback to first port if named ports not found
	}

	k.Logger.Info("Found service", "name", svc.Name, "port", metricsPort)

	// Construct metrics endpoint URL
	metricsEndpoint := fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:%d/proxy/metrics",
		k.K8SConfig.Host, kubeStateMetricsNamespace, kubeStateMetricsName, metricsPort)

	// Check for kube-state-metrics metrics
	return k.checkKubeStateMetrics(ctx, metricsEndpoint)
}

func (k *KubeStateMetricsTest) checkKubeStateMetrics(ctx context.Context, metricsEndpoint string) error {
	k.Logger.Info("Checking for kube-state-metrics metrics", "endpoint", metricsEndpoint)

	// Create HTTP client with proper auth from K8s config
	roundTripper, err := rest.TransportFor(k.K8SConfig)
	if err != nil {
		return fmt.Errorf("failed to create HTTP transport: %v", err)
	}
	httpClient := &http.Client{Transport: roundTripper}

	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		// Create request with context
		req, err := http.NewRequestWithContext(ctx, "GET", metricsEndpoint, nil)
		if err != nil {
			k.Logger.Error(err, "Failed to create HTTP request")
			return false, nil
		}

		// Execute the request
		resp, err := httpClient.Do(req)
		if err != nil {
			k.Logger.Error(err, "Failed to execute HTTP request")
			return false, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			k.Logger.Info("Non-OK status from metrics endpoint", "status", resp.StatusCode)
			return false, nil
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			k.Logger.Error(err, "Failed to read response body")
			return false, nil
		}

		// Check for expected metrics
		return k.checkForExpectedMetrics(string(body)), nil
	})
}

func (k *KubeStateMetricsTest) checkForExpectedMetrics(metricsOutput string) bool {
	// Key kube-state-metrics metrics that should be present
	metricChecks := []string{
		"kube_pod_",
		"kube_deployment_",
		"kube_node_",
		"kube_namespace_",
		"kube_service_",
	}

	for _, metric := range metricChecks {
		if !strings.Contains(metricsOutput, metric) {
			k.Logger.Info("Missing expected metric prefix", "metric", metric)
			return false
		}
		k.Logger.Info("Found expected metric prefix", "metric", metric)
	}

	return true
}

// CollectLogs gathers logs for debugging
func (k *KubeStateMetricsTest) CollectLogs(ctx context.Context) error {
	return k.addon.FetchLogs(ctx, k.K8S, k.Logger)
}

// Delete removes the addon
func (k *KubeStateMetricsTest) Delete(ctx context.Context) error {
	return k.addon.Delete(ctx, k.EKSClient, k.Logger)
}
