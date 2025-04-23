package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		if err := getNodeMetrics(metricsClient, node, logger); err != nil {
			return err
		}
	}

	// pod metrics across all namespaces
	return getPodMetrics(metricsClient, logger)
}

func getNodeMetrics(metricsClient *metricsv1beta1.Clientset, node v1.Node, logger logr.Logger) error {
	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().Get(context.TODO(), node.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting metrics for node %s: %v", node.Name, err)
	}

	logger.Info("\nMetrics for node %s:\n", node.Name)
	fmt.Printf("CPU usage: %v\n", nodeMetrics.Usage.Cpu().String())
	fmt.Printf("Memory usage: %v\n", nodeMetrics.Usage.Memory().String())
	return nil
}

func getPodMetrics(metricsClient *metricsv1beta1.Clientset, logger logr.Logger) error {
	pods, err := metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting pod metrics: %v", err)
	}

	logger.Info("\nFound metrics for %d pods\n", len(pods.Items))
	return nil
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
