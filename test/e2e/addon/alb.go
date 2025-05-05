package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	awsLBCNamespace = "kube-system"
	awsLBCName      = "aws-load-balancer-controller"
	testAppName     = "alb-test-app"
	testNamespace   = "alb-test"
)

type AWSLoadBalancerControllerTest struct {
	Cluster   string
	addon     Addon
	K8S       clientgo.Interface
	EKSClient *eks.Client
	K8SConfig *rest.Config
	Logger    logr.Logger
}

func (a AWSLoadBalancerControllerTest) Run(ctx context.Context) error {
	a.addon = Addon{
		Cluster:   a.Cluster,
		Namespace: awsLBCNamespace,
		Name:      awsLBCName,
	}

	if err := a.Create(ctx); err != nil {
		return err
	}

	if err := a.Validate(ctx); err != nil {
		return err
	}

	return nil
}

func (a AWSLoadBalancerControllerTest) Create(ctx context.Context) error {
	if err := a.addon.CreateAddon(ctx, a.EKSClient, a.K8S, a.Logger); err != nil {
		return err
	}

	if err := kubernetes.WaitForDeploymentReady(ctx, a.Logger, a.K8S, a.addon.Namespace, a.addon.Name); err != nil {
		return err
	}

	return nil
}

func (a AWSLoadBalancerControllerTest) Validate(ctx context.Context) error {
	// 1. Create a test namespace
	_, err := a.K8S.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create test namespace: %v", err)
	}

	a.Logger.Info("Created test namespace", "namespace", testNamespace)

	// 2. Deploy a test application
	// Create a test deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName,
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { r := int32(2); return &r }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": testAppName,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": testAppName,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "web",
							Image: "nginx:latest",
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = a.K8S.AppsV1().Deployments(testNamespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create test deployment: %v", err)
	}

	// 3. Wait for deployment to be ready
	err = kubernetes.WaitForDeploymentReady(ctx, a.Logger, a.K8S, testNamespace, testAppName)
	if err != nil {
		return fmt.Errorf("failed to wait for test deployment to be ready: %v", err)
	}

	// 4. Create a service
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName,
			Namespace: testNamespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   v1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": testAppName,
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}

	_, err = a.K8S.CoreV1().Services(testNamespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create test service: %v", err)
	}

	// 5. Create an ingress with annotations for AWS LBC
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                            "alb",
				"alb.ingress.kubernetes.io/scheme":                       "internal",
				"alb.ingress.kubernetes.io/target-type":                  "ip",
				"alb.ingress.kubernetes.io/healthcheck-path":             "/",
				"alb.ingress.kubernetes.io/healthcheck-protocol":         "HTTP",
				"alb.ingress.kubernetes.io/healthcheck-interval-seconds": "15",
				"alb.ingress.kubernetes.io/healthcheck-timeout-seconds":  "5",
				"alb.ingress.kubernetes.io/success-codes":                "200",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: testAppName,
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = a.K8S.NetworkingV1().Ingresses(testNamespace).Create(ctx, ingress, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create test ingress: %v", err)
	}

	// 6. Wait for the AWS Load Balancer Controller to reconcile the ingress
	err = wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		ing, err := a.K8S.NetworkingV1().Ingresses(testNamespace).Get(ctx, testAppName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check for the ALB address to be populated
		if len(ing.Status.LoadBalancer.Ingress) > 0 && ing.Status.LoadBalancer.Ingress[0].Hostname != "" {
			a.Logger.Info("ALB is provisioned", "hostname", ing.Status.LoadBalancer.Ingress[0].Hostname)
			return true, nil
		}

		a.Logger.Info("Waiting for ALB to be provisioned")
		return false, nil
	})

	if err != nil {
		return err
	}

	a.Logger.Info("AWS Load Balancer Controller validation completed")
	return nil
}

func (a AWSLoadBalancerControllerTest) CollectLogs(ctx context.Context) error {
	return a.addon.FetchLogs(ctx, a.K8S, a.Logger, []string{awsLBCName}, tailLines)
}

func (a AWSLoadBalancerControllerTest) Delete(ctx context.Context) error {
	// Cleanup resources created in the test
	a.Logger.Info("Cleaning up ALB test resources")

	// Delete the test ingress
	if err := a.K8S.NetworkingV1().Ingresses(testNamespace).Delete(ctx, testAppName, metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete test ingress")
	}

	// Delete the service
	if err := a.K8S.CoreV1().Services(testNamespace).Delete(ctx, testAppName, metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete test service")
	}

	// Delete the deployment
	if err := a.K8S.AppsV1().Deployments(testNamespace).Delete(ctx, testAppName, metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete test deployment")
	}

	// Delete the namespace
	if err := a.K8S.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{}); err != nil {
		a.Logger.Error(err, "Failed to delete test namespace")
	}

	// Delete the addon
	return a.addon.Delete(ctx, a.EKSClient, a.Logger)
}
