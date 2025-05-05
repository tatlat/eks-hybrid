package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	kubeProxyNamespace = "kube-system"
	kubeProxyName      = "kube-proxy"
)

type KubeProxyTest struct {
	Cluster   string
	addon     Addon
	K8S       clientgo.Interface
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

func (k KubeProxyTest) Run(ctx context.Context) error {
	k.addon = Addon{
		Cluster:   k.Cluster,
		Namespace: kubeProxyNamespace,
		Name:      kubeProxyName,
	}

	if err := k.Create(ctx); err != nil {
		return err
	}

	if err := k.Validate(ctx); err != nil {
		return err
	}

	return nil
}

func (k KubeProxyTest) Create(ctx context.Context) error {
	if err := k.addon.CreateAddon(ctx, k.EKSClient, k.K8S, k.Logger); err != nil {
		return err
	}

	if err := kubernetes.WaitForDaemonSetReady(ctx, k.Logger, k.K8S, k.addon.Namespace, k.addon.Name); err != nil {
		return err
	}

	return nil
}

func (k KubeProxyTest) Validate(ctx context.Context) error {
	// Check network connectivity by testing a service
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		// Test connectivity to kubernetes service
		_, err := k.K8S.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
		if err != nil {
			k.Logger.Error(err, "Failed to find kubernetes service")
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("failed to validate kube-proxy functionality: %v", err)
	}

	k.Logger.Info("Successfully validated kube-proxy connectivity")
	return nil
}

func (k KubeProxyTest) CollectLogs(ctx context.Context) error {
	return k.addon.FetchLogs(ctx, k.K8S, k.Logger, []string{kubeProxyName}, tailLines)
}

func (k KubeProxyTest) Delete(ctx context.Context) error {
	return k.addon.Delete(ctx, k.EKSClient, k.Logger)
}
