package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	metricsServerNamespace = "kube-system"
	metricsServerName      = "metrics-server"
)

type MetricsServerAddon struct {
	Addon
	cfg *rest.Config
}

func MetricsServerProvider() Provider {
	return Provider{
		Name:        metricsServerName,
		Constructor: NewMetricsServerAddon,
	}
}

func NewMetricsServerAddon(cluster string, cfg *rest.Config) AddonIface {
	return MetricsServerAddon{
		Addon: Addon{
			Cluster:   cluster,
			Name:      metricsServerName,
			Namespace: metricsServerNamespace,
		},
		cfg: cfg,
	}
}

func (m MetricsServerAddon) Setup(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	return nil
}

func (m MetricsServerAddon) PostInstall(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	return nil
}

func (m MetricsServerAddon) Validate(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	metricsClient, err := metricsv1beta1.NewForConfig(m.cfg)
	if err != nil {
		return fmt.Errorf("creating metrics client: %v", err)
	}

	nodes, err := k8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting nodes: %v", err)
	}

	fmt.Printf("Found %d nodes\n", len(nodes.Items))

	for _, node := range nodes.Items {
		if err := getNodeMetrics(ctx, metricsClient, node, logger); err != nil {
			return err
		}
	}

	// pod metrics across all namespaces
	return getPodMetrics(ctx, metricsClient, logger)
}

func getNodeMetrics(ctx context.Context, metricsClient *metricsv1beta1.Clientset, node v1.Node, logger logr.Logger) error {
	consecutiveErrors := 0

	// Add initial check for metrics-server availability
	_, err := metricsClient.Discovery().ServerVersion()
	if err != nil {
		logger.Error(err, "Failed to connect to metrics server")
		return err
	}

	err = wait.PollUntilContextTimeout(ctx, addonDelayInterval, addonWaitTimeout, true, func(ctx context.Context) (done bool, err error) {
		nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().Get(ctx, node.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsServiceUnavailable(err) {
				logger.Info("Metrics server is temporarily unavailable", "error", err)
				consecutiveErrors++
				if consecutiveErrors > 3 {
					return false, fmt.Errorf("metrics server repeatedly unavailable for node %s: %v", node.Name, err)
				}
				return false, nil
			}

			if errors.IsNotFound(err) {
				logger.Info("Metrics not yet available for node", "node", node.Name)
				consecutiveErrors++
				if consecutiveErrors > 3 {
					return false, fmt.Errorf("metrics not found for node %s after multiple attempts: %v", node.Name, err)
				}
				return false, nil
			}

			return false, fmt.Errorf("unexpected error getting metrics for node %s: %v", node.Name, err)
		}

		logger.Info("Retrieved metrics for node",
			"node", node.Name,
			"cpu", nodeMetrics.Usage.Cpu().String(),
			"memory", nodeMetrics.Usage.Memory().String())

		return true, nil
	})

	return err
}

func getPodMetrics(ctx context.Context, metricsClient *metricsv1beta1.Clientset, logger logr.Logger) error {
	consecutiveErrors := 0

	err := wait.PollUntilContextTimeout(ctx, addonDelayInterval, addonWaitTimeout, true, func(ctx context.Context) (done bool, err error) {
		pods, err := metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
		if err != nil {
			consecutiveErrors += 1
			if consecutiveErrors > 3 {
				return false, fmt.Errorf("getting pod metrics: %v", err)
			}
			logger.Info("Retryable error getting pod metrics. Continuing to poll", "error", err)
			return false, nil // continue polling
		}

		logger.Info("\nFound metrics for %d pods\n", len(pods.Items))
		return true, nil
	})

	return err
}

func (m MetricsServerAddon) Cleanup(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	return m.Delete(ctx, eksClient, logger)
}

func (m MetricsServerAddon) GetName() string {
	return metricsServerName
}

func (m MetricsServerAddon) GetNamespace() string {
	return metricsServerNamespace
}

func (m MetricsServerAddon) GetContainerName() string {
	return metricsServerName
}
