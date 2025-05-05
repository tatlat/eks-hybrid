package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	coreDNSNamespace = "kube-system"
	coreDNSName      = "coredns"
	dnsJobName       = "dns-test-job"
)

type CoreDNSTest struct {
	Cluster   string
	addon     Addon
	K8S       clientgo.Interface
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

func (c CoreDNSTest) Run(ctx context.Context) error {
	c.addon = Addon{
		Cluster:   c.Cluster,
		Namespace: coreDNSNamespace,
		Name:      coreDNSName,
	}

	if err := c.Create(ctx); err != nil {
		return err
	}

	if err := c.Validate(ctx); err != nil {
		return err
	}

	return nil
}

func (c CoreDNSTest) Create(ctx context.Context) error {
	if err := c.addon.CreateAddon(ctx, c.EKSClient, c.K8S, c.Logger); err != nil {
		return err
	}

	if err := kubernetes.WaitForDeploymentReady(ctx, c.Logger, c.K8S, c.addon.Namespace, c.addon.Name); err != nil {
		return err
	}

	return nil
}

func (c CoreDNSTest) Validate(ctx context.Context) error {
	// Test DNS resolution functionality by creating a test job

	// Create a test job that performs DNS lookups
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dnsJobName,
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "dns-test",
							Image: "busybox:latest",
							Command: []string{
								"/bin/sh", "-c",
								"nslookup kubernetes.default.svc.cluster.local && nslookup google.com",
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
			BackoffLimit: func() *int32 { i := int32(0); return &i }(),
		},
	}

	if _, err := c.K8S.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create DNS test job: %v", err)
	}

	c.Logger.Info("Created DNS test job")

	// Wait for job to complete
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		job, err := c.K8S.BatchV1().Jobs("default").Get(ctx, dnsJobName, metav1.GetOptions{})
		if err != nil {
			c.Logger.Error(err, "Failed to get DNS test job")
			return false, nil
		}

		if job.Status.Succeeded > 0 {
			c.Logger.Info("DNS test job completed successfully")
			return true, nil
		}

		if job.Status.Failed > 0 {
			return false, fmt.Errorf("DNS test job failed")
		}

		c.Logger.Info("Waiting for DNS test job to complete")
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("DNS resolution test failed: %v", err)
	}

	c.Logger.Info("Successfully validated CoreDNS functionality")
	return nil
}

func (c CoreDNSTest) CollectLogs(ctx context.Context) error {
	// Get logs from the failed job for debugging
	podList, _ := c.K8S.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", dnsJobName),
	})

	if len(podList.Items) > 0 {
		logs, _ := c.K8S.CoreV1().Pods("default").GetLogs(podList.Items[0].Name, &v1.PodLogOptions{}).DoRaw(ctx)
		c.Logger.Info("DNS test job failed", "logs", string(logs))
	}
	return c.addon.FetchLogs(ctx, c.K8S, c.Logger, []string{coreDNSName}, tailLines)
}

func (c CoreDNSTest) Delete(ctx context.Context) error {
	err := c.K8S.BatchV1().Jobs("default").Delete(ctx, dnsJobName, metav1.DeleteOptions{})
	if err != nil {
		c.Logger.Info("could not delete DNS test job: %s", err)
	}
	return c.addon.Delete(ctx, c.EKSClient, c.Logger)
}
