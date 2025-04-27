package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	kube "github.com/aws/eks-hybrid/test/e2e/kubernetes"
)

const (
	nodeMonitoringAgentNamespace = "kube-system"
	nodeMonitoringAgentName      = "metrics-server"
)

type NodeMonitoringAgentAddon struct {
	baseAddon Addon
	cfg       *rest.Config
}

func NodeMonitoringAgentWorkflow() WorkflowProvider {
	return WorkflowProvider{
		Name:        nodeMonitoringAgentName,
		Constructor: NewNodeMonitoringAgentAddon,
	}
}

func NewNodeMonitoringAgentAddon(cluster string, cfg *rest.Config) AddonWorkflow {
	return NodeMonitoringAgentAddon{
		baseAddon: Addon{
			Cluster:   cluster,
			Name:      nodeMonitoringAgentName,
			Namespace: nodeMonitoringAgentNamespace,
		},
		cfg: cfg,
	}
}

func (m NodeMonitoringAgentAddon) Create(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	if err := m.baseAddon.CreateAddon(ctx, eksClient, k8s, logger); err != nil {
		return err
	}

	if err := kube.WaitForDaemonSetReady(ctx, logger, k8s, m.baseAddon.Namespace, m.baseAddon.Name); err != nil {
		return err
	}

	return nil
}

func deployKernelError() error {
	return nil
}

func (m NodeMonitoringAgentAddon) Validate(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	nodes, err := k8s.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		node.Name = ""
	}
	return nil
}

func (m NodeMonitoringAgentAddon) CollectLogs(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	AddonListOptions := getAddonListOptions(m.GetName())
	pods, err := k8s.CoreV1().Pods(m.GetNamespace()).List(ctx, AddonListOptions)
	if err != nil {
		return fmt.Errorf("getting pods for node monitoring agent: %v", err)
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

func (m NodeMonitoringAgentAddon) Delete(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error {
	return m.baseAddon.Delete(ctx, eksClient, logger)
}

func (m NodeMonitoringAgentAddon) GetName() string {
	return nodeMonitoringAgentName
}

func (m NodeMonitoringAgentAddon) GetNamespace() string {
	return nodeMonitoringAgentNamespace
}
