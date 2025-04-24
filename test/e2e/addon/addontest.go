package addon

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	kube "github.com/aws/eks-hybrid/test/e2e/kubernetes"
)

const (
	tailLines          = 10
	addonDelayInterval = 30 * time.Second
	addonWaitTimeout   = 2 * time.Minute
)

type AddonTest struct {
	clientConfig *rest.Config
	k8s          kubernetes.Interface
	eksClient    *eks.Client
	logger       logr.Logger
	addon        AddonIface
}

type Provider struct {
	Name        string
	Constructor Constructor
}

type Constructor func(cluster string, cfg *rest.Config) AddonIface

type AddonIface interface {
	Setup(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	CreateAddon(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	PostInstall(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	Validate(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	Cleanup(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	GetName() string
	GetNamespace() string
	GetContainerName() string
}

func NewAddonTest(clientConfig *rest.Config, k8s kubernetes.Interface, eksClient *eks.Client, logger logr.Logger, addon AddonIface) AddonTest {
	return AddonTest{
		clientConfig: clientConfig,
		k8s:          k8s,
		eksClient:    eksClient,
		logger:       logger,
		addon:        addon,
	}
}

func (a *AddonTest) Cleanup(ctx context.Context) error {
	return a.addon.Cleanup(ctx, a.eksClient, a.k8s, a.logger)
}

func (a *AddonTest) CollectLogs(ctx context.Context) error {
	addonListOptions := getAddonListOptions(a.addon.GetName())
	pods, err := a.k8s.CoreV1().Pods(a.addon.GetNamespace()).List(ctx, addonListOptions)
	if err != nil {
		return fmt.Errorf("getting pods for addon: %v", err)
	}

	for _, pod := range pods.Items {
		logOpts := getPodLogOptions(a.addon.GetContainerName(), aws.Int64(tailLines))
		logs, err := kube.GetPodLogsWithRetries(ctx, a.k8s, pod.Name, pod.Namespace, logOpts)
		if err != nil {
			return err
		}

		a.logger.Info("Logs for pod %s:\n%s\n", pod.Name, logs)
	}

	return nil
}

func (a *AddonTest) Run(ctx context.Context) error {
	if err := a.addon.Setup(ctx, a.eksClient, a.k8s, a.logger); err != nil {
		return err
	}

	if err := a.addon.CreateAddon(ctx, a.eksClient, a.k8s, a.logger); err != nil {
		return err
	}

	if err := a.addon.PostInstall(ctx, a.eksClient, a.k8s, a.logger); err != nil {
		return err
	}

	if err := a.addon.Validate(ctx, a.eksClient, a.k8s, a.logger); err != nil {
		return err
	}

	return nil
}
