package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"

	kube "github.com/aws/eks-hybrid/test/e2e/kubernetes"
)

const (
	metricsServerNamespace = "kube-system"
	metricsServerName      = "metrics-server"
)

type MetricsServerAddon struct {
	baseAddon Addon
	cfg       *rest.Config
}

func MetricsServerWorkflow() WorkflowProvider {
	return WorkflowProvider{
		Name:        metricsServerName,
		Constructor: NewMetricsServerAddon,
	}
}

func NewMetricsServerAddon(cluster string, cfg *rest.Config) AddonWorkflow {
	return MetricsServerAddon{
		baseAddon: Addon{
			Cluster:   cluster,
			Name:      metricsServerName,
			Namespace: metricsServerNamespace,
		},
		cfg: cfg,
	}
}

func (m MetricsServerAddon) Create(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	if err := m.baseAddon.CreateAddon(ctx, eksClient, k8s, logger); err != nil {
		return err
	}

	if err := kube.WaitForDeploymentReady(ctx, logger, k8s, m.baseAddon.Namespace, m.baseAddon.Name); err != nil {
		return err
	}

	return nil
}

func (m MetricsServerAddon) Validate(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	metricsClient, err := metricsv1beta1.NewForConfig(m.cfg)
	if err != nil {
		return fmt.Errorf("creating metrics client: %v", err)
	}

	if err := getNodeMetrics(ctx, metricsClient, logger); err != nil {
		return err
	}

	// pod metrics across all namespaces
	return getPodMetrics(ctx, metricsClient, logger)
}

func (m MetricsServerAddon) CollectLogs(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	AddonListOptions := getAddonListOptions(m.GetName())
	pods, err := k8s.CoreV1().Pods(m.GetNamespace()).List(ctx, AddonListOptions)
	if err != nil {
		return fmt.Errorf("getting pods for metrics-server: %v", err)
	}

	for _, pod := range pods.Items {
		logOpts := getPodLogOptions(m.GetName(), aws.Int64(tailLines))
		logs, err := kube.GetPodLogsWithRetries(ctx, k8s, pod.Name, pod.Namespace, logOpts)
		if err != nil {
			return err
		}

		logger.Info("Logs for pod\n\n", pod.Name, fmt.Sprintf("%s\n\n", logs))
	}

	return nil
}

func getNodeMetrics(ctx context.Context, metricsClient *metricsv1beta1.Clientset, logger logr.Logger) error {
	consecutiveErrors := 0

	// Add initial check for metrics-server availability
	_, err := metricsClient.Discovery().ServerVersion()
	if err != nil {
		logger.Error(err, "Failed to connect to metrics server")
		return err
	}

	err = wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		allNodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		if err != nil {
			if errors.IsServiceUnavailable(err) {
				logger.Info("Metrics server is temporarily unavailable", "error", err)
				consecutiveErrors++
				if consecutiveErrors > addonMaxRetries {
					return false, fmt.Errorf("metrics server repeatedly unavailable: %v", err)
				}
				return false, nil
			}

			return false, fmt.Errorf("unexpected error getting metrics: %v", err)
		}
		for _, metrics := range allNodeMetrics.Items {
			logger.Info("Found metrics for node", "node", metrics.Name,
				"cpu", metrics.Usage.Cpu().String(),
				"memory", metrics.Usage.Memory().String(),
			)
		}

		return true, nil
	})

	return err
}

func getPodMetrics(ctx context.Context, metricsClient *metricsv1beta1.Clientset, logger logr.Logger) error {
	consecutiveErrors := 0

	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		pods, err := metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
		if err != nil {
			consecutiveErrors += 1
			if consecutiveErrors > addonMaxRetries {
				return false, fmt.Errorf("getting pod metrics: %v", err)
			}
			logger.Info("Retryable error getting pod metrics. Continuing to poll", "error", err)
			return false, nil // continue polling
		}

		logger.Info(fmt.Sprintf("Found metrics for %d pods", len(pods.Items)))
		return true, nil
	})

	return err
}

func (m MetricsServerAddon) Delete(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	return m.baseAddon.Delete(ctx, eksClient, logger)
}

func (m MetricsServerAddon) GetName() string {
	return metricsServerName
}

func (m MetricsServerAddon) GetNamespace() string {
	return metricsServerNamespace
}
