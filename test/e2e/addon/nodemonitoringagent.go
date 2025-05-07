package addon

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	nodeMonitoringAgentNamespace = "kube-system"
	nodeMonitoringAgentName      = "metrics-server"
)

type NodeMonitoringAgentTest struct {
	Cluster   string
	addon     Addon
	K8S       clientgo.Interface
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

func (n NodeMonitoringAgentTest) Create(ctx context.Context) error {
	if err := n.addon.CreateAddon(ctx, n.EKSClient, n.K8S, n.Logger); err != nil {
		return err
	}

	if err := kubernetes.WaitForDaemonSetReady(ctx, n.Logger, n.K8S, n.addon.Namespace, n.addon.Name); err != nil {
		return err
	}

	if err := deployKernelError(ctx, n.K8S, n.Logger); err != nil {
		return err
	}

	return nil
}

func deployKernelError(ctx context.Context, k8s clientgo.Interface, logger logr.Logger) error {
	return nil
}

func (n NodeMonitoringAgentTest) Validate(ctx context.Context) error {
	var nodes *v1.NodeList

	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		var err error
		nodes, err = n.K8S.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			n.Logger.Error(err, "Failed to list nodes")
			return false, nil // Return false to retry, nil to not stop with error
		}
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("failed to list nodes after retries: %v", err)
	}

	for _, node := range nodes.Items {
		nodeLogger := n.Logger.WithValues("node", node.Name)
		fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Node", node.Name)

		var events *v1.EventList
		err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
			var err error
			events, err = n.K8S.CoreV1().Events("").List(ctx, metav1.ListOptions{
				FieldSelector: fieldSelector,
			})
			if err != nil {
				nodeLogger.Error(err, "Failed to get events")
				return false, nil
			}
			return true, nil
		})

		if err != nil {
			return fmt.Errorf("failed to get events for node %s after retries: %v", node.Name, err)
		}

		foundMonitoringEvents := false
		for _, event := range events.Items {
			if isMonitoringAgentEvent(event) {
				foundMonitoringEvents = true
				nodeLogger.Info("Found monitoring agent event",
					"message", event.Message,
					"reason", event.Reason,
					"count", event.Count)
			}
		}

		if !foundMonitoringEvents {
			nodeLogger.Info("No monitoring agent events found")
		}
	}

	return nil
}

func isMonitoringAgentEvent(event v1.Event) bool {
	return strings.Contains(strings.ToLower(event.Source.Component), nodeMonitoringAgentName)
}

func (n NodeMonitoringAgentTest) CollectLogs(ctx context.Context) error {
	return n.addon.FetchLogs(ctx, n.K8S, n.Logger)
}

func (n NodeMonitoringAgentTest) Delete(ctx context.Context) error {
	return n.addon.Delete(ctx, n.EKSClient, n.Logger)
}
