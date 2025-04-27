package addon

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	tailLines       = 10
	addonMaxRetries = 5
)

var (
	retryBackoff = wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    5,
		Cap:      30 * time.Second,
	}
)

type AddonTest struct {
	ClientConfig *rest.Config
	K8s          kubernetes.Interface
	EksClient    *eks.Client
	Logger       logr.Logger
	Workflow     AddonWorkflow
}

type WorkflowProvider struct {
	Name        string
	Constructor WorkflowConstructor
}

// WorkflowConstructor is a function that returns an AddonWorkflow.
type WorkflowConstructor func(cluster string, cfg *rest.Config) AddonWorkflow

// AddonWorkflow defines the workflows that happen during an addon test.
type AddonWorkflow interface {
	// Create installs the addon along with other resources and waits for it to become ready.
	Create(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	// Validate checks if the addon functions.
	Validate(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	// Delete removes any resources that were created by this workflow.
	Delete(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	// CollectLogs retrieves key logs from the addon's pods and writes them to the logger for debugging.
	CollectLogs(ctx context.Context, eksClient *eks.Client, k8s kubernetes.Interface, logger logr.Logger) error
	// GetName returns the name of the addon.
	GetName() string
	// GetNamespace returns the namespace of the addon.
	GetNamespace() string
}

func (a *AddonTest) CollectLogs(ctx context.Context) error {
	return a.Workflow.CollectLogs(ctx, a.EksClient, a.K8s, a.Logger)
}

func (a *AddonTest) Cleanup(ctx context.Context) error {
	return a.Workflow.Delete(ctx, a.EksClient, a.K8s, a.Logger)
}

func (a *AddonTest) Run(ctx context.Context) error {
	if err := a.Workflow.Create(ctx, a.EksClient, a.K8s, a.Logger); err != nil {
		return err
	}

	if err := a.Workflow.Validate(ctx, a.EksClient, a.K8s, a.Logger); err != nil {
		return err
	}

	return nil
}
