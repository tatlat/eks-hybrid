package node

import (
	"context"
	"fmt"
	"github.com/aws/eks-hybrid/internal/kubelet"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultStaticPodManifestPath = "/etc/kubernetes/manifest"

func IsUnscheduled(ctx context.Context) error {
	nodeName, err := kubelet.GetNodeName()
	if err != nil {
		return err
	}

	clientset, err := kubelet.GetKubeClientFromKubeConfig()
	if err != nil {
		return err
	}

	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !node.Spec.Unschedulable {
		return fmt.Errorf("node is schedulable. Please drain the node or use --skip node-validation")
	}
	return nil
}

func IsDrained(ctx context.Context) error {
	podsOnNode, err := getPodsOnNode()
	if err != nil {
		return errors.Wrap(err, "failed to get pods on node")
	}

	for _, filter := range getDrainedPodFilters() {
		podsOnNode, err = filter(podsOnNode)
		if err != nil {
			return errors.Wrap(err, "running filter on pods")
		}
	}
	if len(podsOnNode) != 0 {
		return fmt.Errorf("only static pods and pods controlled by daemon-sets can be running on the node. Please move pods " +
			"to different node or provide --skip pod-validation")
	}
	return nil
}
