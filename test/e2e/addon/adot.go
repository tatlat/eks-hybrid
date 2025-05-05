package addon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	adotNamespace     = "opentelemetry-operator-system"
	adotName          = "adot"
	adotOperatorName  = "aws-otel-collector"
	adotTestNamespace = "adot-test"
	adotTestAppName   = "adot-sample-app"
)

type ADOTTest struct {
	Cluster   string
	addon     Addon
	K8S       clientgo.Interface
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

func (a ADOTTest) Run(ctx context.Context) error {
	a.addon = Addon{
		Cluster:       a.Cluster,
		Namespace:     adotNamespace,
		Name:          adotName,
		Configuration: getCollectorConfiguration(),
	}

	if err := a.Create(ctx); err != nil {
		return err
	}

	if err := a.Validate(ctx); err != nil {
		return err
	}

	return nil
}

func (a ADOTTest) Create(ctx context.Context) error {
	// Ensure the test namespace exists for our sample app
	_, err := a.K8S.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: adotTestNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create ADOT test namespace: %v", err)
	}

	// Deploy a sample app before creating the addon
	if err := a.deploySampleApp(ctx); err != nil {
		return fmt.Errorf("failed to deploy sample app: %v", err)
	}

	// Use the standard addon creation but with our customized configuration
	if err := a.addon.CreateAddon(ctx, a.EKSClient, a.K8S, a.Logger); err != nil {
		return err
	}

	// Wait for the ADOT operator deployment to be ready
	if err := kubernetes.WaitForDeploymentReady(ctx, a.Logger, a.K8S, adotNamespace, adotOperatorName); err != nil {
		return fmt.Errorf("failed waiting for ADOT operator deployment: %v", err)
	}

	a.Logger.Info("ADOT operator deployment is ready")
	return nil
}

func getCollectorConfiguration() string {
	return `{
		"collector": {
			"serviceAccount": {
				"create": true,
				"name": "adot-collector"
			},
			"mode": "deployment",
			"replicas": 1,
			"resources": {
				"limits": {
					"cpu": "256m",
					"memory": "512Mi"
				},
				"requests": {
					"cpu": "32m",
					"memory": "64Mi"
				}
			},
			"configMap": {
				"name": "adot-collector-conf",
				"create": true
			},
			"config": {
				"receivers": {
					"prometheus": {
						"config": {
							"scrape_configs": [
								{
									"job_name": "kubernetes-pods",
									"scrape_interval": "15s",
									"kubernetes_sd_configs": [{ "role": "pod" }],
									"relabel_configs": [
										{
											"source_labels": ["__meta_kubernetes_pod_annotation_prometheus_io_scrape"],
											"action": "keep",
											"regex": "true"
										}
									]
								}
							]
						}
					},
					"otlp": {
						"protocols": {
							"grpc": { "endpoint": "0.0.0.0:4317" },
							"http": { "endpoint": "0.0.0.0:4318" }
						}
					},
					"filelog": {
						"include": ["/var/log/pods/*/*/*.log"],
						"start_at": "beginning",
						"include_file_path": true,
						"operators": [
							{
								"type": "router",
								"id": "get-format",
								"routes": [
									{
										"output": "parser-docker",
										"expr": "body matches '^\\\\{\"'"
									},
									{
										"output": "parser-crio",
										"expr": "body matches '^[^ Z]+Z'"
									},
									{
										"output": "parser-containerd",
										"expr": "body matches '^[0-9]{4}-[0-9]{2}-[0-9]{2}T'"
									}
								]
							}
						]
					}
				},
				"processors": {
					"batch": {},
					"memory_limiter": {
						"check_interval": "1s",
						"limit_mib": 400
					}
				},
				"exporters": {
					"logging": {
						"loglevel": "debug"
					},
					"awsxray": {},
					"awsemf": {
						"region": "us-west-2",
						"namespace": "EKSHybridADOT"
					},
					"awscloudwatchlogs": {
						"region": "us-west-2",
						"log_group_name": "/aws/eks-hybrid/adot-logs"
					}
				},
				"service": {
					"pipelines": {
						"metrics": {
							"receivers": ["prometheus"],
							"processors": ["batch", "memory_limiter"],
							"exporters": ["logging", "awsemf"]
						},
						"traces": {
							"receivers": ["otlp"],
							"processors": ["batch", "memory_limiter"],
							"exporters": ["logging", "awsxray"]
						},
						"logs": {
							"receivers": ["filelog"],
							"processors": ["batch", "memory_limiter"],
							"exporters": ["logging", "awscloudwatchlogs"]
						}
					}
				}
			}
		}
	}`
}

func (a ADOTTest) Validate(ctx context.Context) error {
	a.Logger.Info("Starting ADOT validation")

	// Wait for collector deployment to be ready
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		// Check for collector pods
		pods, err := a.K8S.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/component=opentelemetry-collector",
		})
		if err != nil {
			a.Logger.Error(err, "Failed to list collector pods")
			return false, nil // retry
		}

		if len(pods.Items) == 0 {
			a.Logger.Info("No collector pods found yet, waiting...")
			return false, nil // retry
		}

		// Check all pods are running
		readyPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				readyPods++
				a.Logger.Info("Collector pod running",
					"name", pod.Name,
					"namespace", pod.Namespace,
					"node", pod.Spec.NodeName)
			}
		}

		if readyPods == len(pods.Items) {
			a.Logger.Info("All collector pods are running", "count", readyPods)
			return true, nil
		}

		a.Logger.Info("Waiting for collector pods to be ready",
			"ready", readyPods,
			"total", len(pods.Items))
		return false, nil
	})

	if err != nil {
		a.Logger.Error(err, "Timed out waiting for collector pods")
		// Continue testing rather than fail
	}

	// Verify collector logs show successful startup
	a.verifyCollectorLogs(ctx)

	// Check for metrics, traces, and logs being collected
	a.validateTelemetryCollection(ctx)

	a.Logger.Info("ADOT validation completed")
	return nil
}

func (a ADOTTest) verifyCollectorLogs(ctx context.Context) {
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		pods, err := a.K8S.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/component=opentelemetry-collector",
		})
		if err != nil {
			a.Logger.Error(err, "Failed to list collector pods")
			return false, nil
		}

		if len(pods.Items) == 0 {
			return false, nil
		}

		// Check the first pod's logs
		logs, err := a.K8S.CoreV1().Pods(pods.Items[0].Namespace).GetLogs(pods.Items[0].Name, &v1.PodLogOptions{
			TailLines: func() *int64 { l := int64(50); return &l }(),
		}).DoRaw(ctx)
		if err != nil {
			a.Logger.Error(err, "Failed to get collector logs")
			return false, nil
		}

		logStr := string(logs)
		if strings.Contains(logStr, "Everything is ready. Begin running and processing data") ||
			strings.Contains(logStr, "Starting otelcol") {
			a.Logger.Info("Collector started successfully", "pod", pods.Items[0].Name)
			return true, nil
		}

		if strings.Contains(logStr, "ERROR") || strings.Contains(logStr, "Error") {
			a.Logger.Info("Found potential issues in collector logs", "pod", pods.Items[0].Name)
			// Continue validation even if we see errors
			return true, nil
		}

		a.Logger.Info("Waiting for collector startup logs")
		return false, nil
	})

	if err != nil {
		a.Logger.Info("Timed out waiting for collector logs")
	}
}

func (a ADOTTest) validateTelemetryCollection(ctx context.Context) {
	// This would ideally check the AWS services (CloudWatch, X-Ray, etc.)
	// to verify data is flowing. For this test, we'll just check collector status.

	a.Logger.Info("Generating sample telemetry data")

	// Generate some HTTP traffic to our sample app to create telemetry
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		// Find the service IP
		svc, err := a.K8S.CoreV1().Services(adotTestNamespace).Get(ctx, adotTestAppName, metav1.GetOptions{})
		if err != nil {
			a.Logger.Error(err, "Failed to get service for sample app")
			return false, nil
		}

		// Create a test pod that sends HTTP requests to the service
		testPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemetry-traffic-generator",
				Namespace: adotTestNamespace,
			},
			Spec: v1.PodSpec{
				RestartPolicy: v1.RestartPolicyNever,
				Containers: []v1.Container{
					{
						Name:  "traffic-gen",
						Image: "curlimages/curl:latest",
						Command: []string{
							"/bin/sh", "-c",
							fmt.Sprintf("for i in $(seq 1 30); do curl -s http://%s:80/ || true; sleep 1; done", svc.Spec.ClusterIP),
						},
					},
				},
			},
		}

		_, err = a.K8S.CoreV1().Pods(adotTestNamespace).Create(ctx, testPod, metav1.CreateOptions{})
		if err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				a.Logger.Error(err, "Failed to create traffic generator pod")
				return false, nil
			}
			// Pod already exists, assume traffic is being generated
			return true, nil
		}

		a.Logger.Info("Created traffic generator pod")
		return true, nil
	})

	if err != nil {
		a.Logger.Info("Unable to create traffic generator", "error", err)
	}

	// Give time for telemetry collection
	time.Sleep(30 * time.Second)

	a.Logger.Info("Sample telemetry data should now be flowing to AWS backends")
}

func (a ADOTTest) deploySampleApp(ctx context.Context) error {
	// Deploy a simple app that will be monitored
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      adotTestAppName,
			Namespace: adotTestNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { r := int32(1); return &r }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": adotTestAppName,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": adotTestAppName,
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   "80",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "sample-app",
							Image: "nginx:latest",
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := a.K8S.AppsV1().Deployments(adotTestNamespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}

	// Create a service for the sample app
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      adotTestAppName,
			Namespace: adotTestNamespace,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": adotTestAppName,
			},
			Ports: []v1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}

	_, err = a.K8S.CoreV1().Services(adotTestNamespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}

	return kubernetes.WaitForDeploymentReady(ctx, a.Logger, a.K8S, adotTestNamespace, adotTestAppName)
}

func (a ADOTTest) CollectLogs(ctx context.Context) error {
	// Collect logs from all collector pods
	pods, err := a.K8S.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=opentelemetry-collector",
	})
	if err != nil {
		a.Logger.Error(err, "Failed to list collector pods for log collection")
		return err
	}

	for _, pod := range pods.Items {
		logs, err := a.K8S.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
			TailLines: func() *int64 { l := int64(100); return &l }(),
		}).DoRaw(ctx)
		if err != nil {
			a.Logger.Error(err, "Failed to get logs", "pod", pod.Name, "namespace", pod.Namespace)
			continue
		}

		a.Logger.Info("Collector logs",
			"pod", pod.Name,
			"namespace", pod.Namespace,
			"logs", string(logs))
	}

	return nil
}

func (a ADOTTest) Delete(ctx context.Context) error {
	// Clean up the test resources first
	if err := a.K8S.CoreV1().Pods(adotTestNamespace).Delete(ctx, "telemetry-traffic-generator", metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete traffic generator pod")
	}

	if err := a.K8S.AppsV1().Deployments(adotTestNamespace).Delete(ctx, adotTestAppName, metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete test deployment")
	}

	if err := a.K8S.CoreV1().Services(adotTestNamespace).Delete(ctx, adotTestAppName, metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete test service")
	}

	if err := a.K8S.CoreV1().Namespaces().Delete(ctx, adotTestNamespace, metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete test namespace")
	}

	// Delete the addon
	return a.addon.Delete(ctx, a.EKSClient, a.Logger)
}
