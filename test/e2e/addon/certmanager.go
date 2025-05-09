package addon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	awspcav1beta1 "github.com/cert-manager/aws-privateca-issuer/pkg/api/v1beta1"
	awspcaclientset "github.com/cert-manager/aws-privateca-issuer/pkg/clientset/v1beta1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	certManagerNamespace      = "cert-manager"
	certManagerName           = "cert-manager"
	certManagerCainjectorName = "cert-manager-cainjector"
	certManagerWebhookName    = "cert-manager-webhook"
	certName                  = "test-cert"
	certTestNamespace         = "cert-test"
	issuerName                = "selfsigned-issuer"
	certSecretName            = "selfsigned-cert-tls"

	// AWS PCA Issuer constants
	awsPcaIssuerNamespace   = "aws-pca-issuer"
	awsPcaIssuerHelmRepo    = "https://cert-manager.github.io/aws-privateca-issuer"
	awsPcaIssuerHelmChart   = "aws-privateca-issuer"
	awsPcaIssuerReleaseName = "aws-pca-issuer"
	awsPcaIssuerName        = "aws-pca-issuer"
	awsPcaCertName          = "aws-pca-test-cert"
	awsPcaCertSecretName    = "aws-pca-cert-tls"
	awsPcaArn               = "arn:aws:acm-pca:region:account:certificate-authority/id" // This should be configured based on actual ARN

	helmTimeout = 300 * time.Second
)

type CertificateData struct {
	CSR        []byte
	PrivateKey []byte
}

type CertManagerTest struct {
	Cluster      string
	addon        *Addon
	K8S          clientgo.Interface
	EKSClient    *eks.Client
	K8SConfig    *rest.Config
	Logger       logr.Logger
	CertClient   certmanagerclientset.Interface
	AwsPcaClient awspcaclientset.Interface
}

func (c *CertManagerTest) Run(ctx context.Context) error {
	c.addon = &Addon{
		Cluster:   c.Cluster,
		Namespace: certManagerNamespace,
		Name:      certManagerName,
	}

	// Initialize AWS PCA client
	var err error
	c.AwsPcaClient, err = awspcaclientset.NewForConfig(c.K8SConfig)
	if err != nil {
		return fmt.Errorf("failed to create AWS PCA client: %v", err)
	}

	if err := c.Create(ctx); err != nil {
		return err
	}

	if err := c.Validate(ctx); err != nil {
		return err
	}

	return nil
}

func (c *CertManagerTest) Create(ctx context.Context) error {
	if err := c.addon.CreateAddon(ctx, c.EKSClient, c.K8S, c.Logger); err != nil {
		return err
	}

	// Wait for cert-manager deployment to be ready
	if err := kubernetes.WaitForDeploymentReady(ctx, c.Logger, c.K8S, certManagerNamespace, certManagerName); err != nil {
		return err
	}

	// Wait for cert-manager webhook deployment to be ready
	if err := kubernetes.WaitForDeploymentReady(ctx, c.Logger, c.K8S, certManagerNamespace, certManagerWebhookName); err != nil {
		return err
	}

	// Wait for cert-manager cainjector deployment to be ready
	if err := kubernetes.WaitForDeploymentReady(ctx, c.Logger, c.K8S, certManagerNamespace, certManagerCainjectorName); err != nil {
		return err
	}

	// Create test namespace
	if _, err := c.K8S.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: certTestNamespace,
		},
	}, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Install AWS PCA issuer using Helm
	if err := c.installAwsPcaIssuer(ctx); err != nil {
		return fmt.Errorf("failed to install AWS PCA issuer: %v", err)
	}

	// Wait for AWS PCA issuer deployment to be ready
	if err := kubernetes.WaitForDeploymentReady(ctx, c.Logger, c.K8S, awsPcaIssuerNamespace, awsPcaIssuerName); err != nil {
		return err
	}

	c.Logger.Info("All cert-manager and AWS PCA issuer components are ready")
	return nil
}

func (c *CertManagerTest) installAwsPcaIssuer(ctx context.Context) error {
	c.Logger.Info("Installing AWS PCA issuer using Helm")

	// Create AWS PCA issuer namespace if it doesn't exist
	if _, err := c.K8S.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: awsPcaIssuerNamespace,
		},
	}, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Initialize Helm configuration
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), awsPcaIssuerNamespace, os.Getenv("HELM_DRIVER"), c.Logger.Info); err != nil {
		return fmt.Errorf("failed to initialize Helm configuration: %v", err)
	}

	// Create Helm client
	client := action.NewInstall(actionConfig)
	client.ReleaseName = awsPcaIssuerReleaseName
	client.Namespace = awsPcaIssuerNamespace
	client.CreateNamespace = true
	client.Wait = true
	client.Timeout = helmTimeout

	// Add Helm repository
	addRepo := action.NewRepoAdd(actionConfig)
	addRepo.Name = "aws-privateca-issuer"
	addRepo.URL = awsPcaIssuerHelmRepo
	if err := addRepo.Run(); err != nil {
		return fmt.Errorf("failed to add Helm repository: %v", err)
	}

	// Update Helm repositories
	updateRepo := action.NewRepoUpdate(actionConfig)
	if _, err := updateRepo.Run(); err != nil {
		return fmt.Errorf("failed to update Helm repositories: %v", err)
	}

	// Set values for the Helm chart
	valueOpts := &values.Options{}
	vals, err := valueOpts.MergeValues()
	if err != nil {
		return fmt.Errorf("failed to merge Helm values: %v", err)
	}

	// Install the Helm chart
	chartPath, err := client.ChartPathOptions.LocateChart(awsPcaIssuerHelmChart, settings)
	if err != nil {
		return fmt.Errorf("failed to locate Helm chart: %v", err)
	}

	chart, err := client.LoadChart(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load Helm chart: %v", err)
	}

	_, err = client.Run(chart, vals)
	if err != nil {
		return fmt.Errorf("failed to install Helm chart: %v", err)
	}

	c.Logger.Info("Successfully installed AWS PCA issuer using Helm")
	return nil
}

func (c *CertManagerTest) Validate(ctx context.Context) error {
	c.Logger.Info("Starting cert-manager validation")

	// Step 1: Create self-signed issuer with retries
	if err := createSelfSignedIssuer(ctx, c.Logger, c.CertClient); err != nil {
		return fmt.Errorf("failed to create self-signed issuer: %v", err)
	}

	// Step 2: Create certificate with retries
	if err := createCertificate(ctx, c.Logger, c.CertClient, certTestNamespace, certName); err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}

	// Step 3: check certificate status
	if err := validateCertificate(ctx, c.Logger, c.CertClient, certTestNamespace, certName); err != nil {
		return fmt.Errorf("Failed to validate certificate: %v", err)
	}

	// Step 4: Create AWS PCA issuer
	if err := c.createAwsPcaIssuer(ctx); err != nil {
		return fmt.Errorf("failed to create AWS PCA issuer: %v", err)
	}

	// Step 5: Create certificate using AWS PCA issuer
	if err := c.createAwsPcaCertificate(ctx); err != nil {
		return fmt.Errorf("failed to create certificate using AWS PCA issuer: %v", err)
	}

	// Step 6: Validate AWS PCA certificate
	if err := c.validateAwsPcaCertificate(ctx); err != nil {
		return fmt.Errorf("failed to validate AWS PCA certificate: %v", err)
	}

	c.Logger.Info("Cert-manager and AWS PCA issuer validation completed")
	return nil
}

func (c *CertManagerTest) createAwsPcaIssuer(ctx context.Context) error {
	c.Logger.Info("Creating AWS PCA issuer")

	awsPcaIssuer := &awspcav1beta1.AWSPCAIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      awsPcaIssuerName,
			Namespace: certTestNamespace,
		},
		Spec: awspcav1beta1.AWSPCAIssuerSpec{
			Arn:       awsPcaArn,
			Region:    "us-west-2", // Configure as needed
			SecretRef: awspcav1beta1.AWSCredentialsSecretReference{
				// Secret containing AWS credentials
			},
		},
	}

	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		_, err := c.AwsPcaClient.AWSPCAIssuers(certTestNamespace).Create(ctx, awsPcaIssuer, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			c.Logger.Error(err, "failed to create AWS PCA issuer, retrying")
			return false, nil
		}

		c.Logger.Info("Created AWS PCA issuer successfully")
		return true, nil
	})
}

func (c *CertManagerTest) createAwsPcaCertificate(ctx context.Context) error {
	c.Logger.Info("Creating certificate using AWS PCA issuer")

	awsPcaCert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      awsPcaCertName,
			Namespace: certTestNamespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			DNSNames: []string{
				"awspca-example.com",
			},
			SecretName: awsPcaCertSecretName,
			IssuerRef: cmmeta.ObjectReference{
				Name:  awsPcaIssuerName,
				Kind:  "AWSPCAIssuer",
				Group: "awspca.cert-manager.io",
			},
		},
	}

	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		_, err := c.CertClient.CertmanagerV1().Certificates(certTestNamespace).Create(ctx, awsPcaCert, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			c.Logger.Error(err, "failed to create AWS PCA certificate, retrying")
			return false, nil
		}

		c.Logger.Info("Created AWS PCA certificate successfully")
		return true, nil
	})
}

func (c *CertManagerTest) validateAwsPcaCertificate(ctx context.Context) error {
	c.Logger.Info("Validating AWS PCA certificate")

	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		cert, err := c.CertClient.CertmanagerV1().Certificates(certTestNamespace).Get(ctx, awsPcaCertName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error getting AWS PCA certificate: %v", err)
		}

		for _, condition := range cert.Status.Conditions {
			if condition.Type == certmanagerv1.CertificateConditionReady && condition.Status == cmmeta.ConditionTrue {
				c.Logger.Info("AWS PCA certificate validated successfully")
				return true, nil
			}
		}

		c.Logger.Info("AWS PCA certificate not valid yet")
		return false, nil
	})
}

func (c *CertManagerTest) CollectLogs(ctx context.Context) error {
	// Collect logs from all cert-manager components
	if err := c.addon.FetchLogs(ctx, c.K8S, c.Logger); err != nil {
		c.Logger.Error(err, "failed to collect logs for cert-manager")
	}

	// Collect logs from AWS PCA issuer components
	if err := c.collectAwsPcaIssuerLogs(ctx); err != nil {
		c.Logger.Error(err, "failed to collect logs for AWS PCA issuer")
	}

	return nil
}

func (c *CertManagerTest) collectAwsPcaIssuerLogs(ctx context.Context) error {
	c.Logger.Info("Collecting AWS PCA issuer logs")

	// Get pods in AWS PCA issuer namespace
	pods, err := c.K8S.CoreV1().Pods(awsPcaIssuerNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods in AWS PCA issuer namespace: %v", err)
	}

	// Create logs directory if it doesn't exist
	logsDir := filepath.Join("logs", "aws-pca-issuer")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Collect logs from each pod
	for _, pod := range pods.Items {
		logs, err := c.K8S.CoreV1().Pods(awsPcaIssuerNamespace).GetLogs(pod.Name, &v1.PodLogOptions{}).Do(ctx).Raw()
		if err != nil {
			c.Logger.Error(err, "failed to get logs for pod", "pod", pod.Name)
			continue
		}

		logFilePath := filepath.Join(logsDir, pod.Name+".log")
		if err := os.WriteFile(logFilePath, logs, 0644); err != nil {
			c.Logger.Error(err, "failed to write logs to file", "pod", pod.Name)
			continue
		}
	}

	return nil
}

func (c *CertManagerTest) Delete(ctx context.Context) error {
	c.Logger.Info("starting cleanup of all test resources")

	// Clean up AWS PCA issuer resources
	if err := c.cleanupAwsPcaIssuerResources(ctx); err != nil {
		c.Logger.Error(err, "failed to clean up AWS PCA issuer resources")
	}

	// Clean up cert-manager test resources
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		err := c.CertClient.CertmanagerV1().CertificateRequests(certTestNamespace).Delete(
			ctx, certName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			c.Logger.Error(err, "failed to delete CertificateRequest, retrying")
			return false, nil
		}

		err = c.K8S.CoreV1().Secrets(certTestNamespace).Delete(ctx, certName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			c.Logger.Error(err, "failed to delete Secret, retrying", "name", certName)
			return false, nil
		}

		err = c.CertClient.CertmanagerV1().Issuers(certTestNamespace).Delete(
			ctx, issuerName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			c.Logger.Error(err, "failed to delete Issuer, retrying", "name", issuerName)
			return false, nil
		}

		err = c.K8S.CoreV1().Namespaces().Delete(ctx, certTestNamespace, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			c.Logger.Error(err, "failed to delete test namespace, retrying", "name", certTestNamespace)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		c.Logger.Error(err, "failed to delete cert-manager k8s resources")
	}

	// Uninstall AWS PCA issuer
	if err := c.uninstallAwsPcaIssuer(ctx); err != nil {
		c.Logger.Error(err, "failed to uninstall AWS PCA issuer")
	}

	// Delete cert-manager addon
	if err := c.addon.Delete(ctx, c.EKSClient, c.Logger); err != nil {
		return err
	}

	c.Logger.Info("successfully cleaned up all cert-manager and AWS PCA issuer test resources")
	return nil
}

func (c *CertManagerTest) cleanupAwsPcaIssuerResources(ctx context.Context) error {
	c.Logger.Info("Cleaning up AWS PCA issuer resources")

	// Delete AWS PCA certificate
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		err := c.CertClient.CertmanagerV1().Certificates(certTestNamespace).Delete(ctx, awsPcaCertName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			c.Logger.Error(err, "failed to delete AWS PCA certificate, retrying")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	// Delete AWS PCA issuer
	err = wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		err := c.AwsPcaClient.AWSPCAIssuers(certTestNamespace).Delete(ctx, awsPcaIssuerName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			c.Logger.Error(err, "failed to delete AWS PCA issuer, retrying")
			return false, nil
		}
		return true, nil
	})

	return err
}

func (c *CertManagerTest) uninstallAwsPcaIssuer(ctx context.Context) error {
	c.Logger.Info("Uninstalling AWS PCA issuer")

	// Initialize Helm configuration
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), awsPcaIssuerNamespace, os.Getenv("HELM_DRIVER"), c.Logger.Info); err != nil {
		return fmt.Errorf("failed to initialize Helm configuration: %v", err)
	}

	// Create Helm uninstall client
	client := action.NewUninstall(actionConfig)
	client.Wait = true
	client.Timeout = helmTimeout

	// Uninstall the Helm chart
	_, err := client.Run(awsPcaIssuerReleaseName)
	if err != nil {
		return fmt.Errorf("failed to uninstall AWS PCA issuer: %v", err)
	}

	// Delete AWS PCA issuer namespace
	err = c.K8S.CoreV1().Namespaces().Delete(ctx, awsPcaIssuerNamespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete AWS PCA issuer namespace: %v", err)
	}

	c.Logger.Info("Successfully uninstalled AWS PCA issuer")
	return nil
}

func createSelfSignedIssuer(ctx context.Context, logger logr.Logger, certClient certmanagerclientset.Interface) error {
	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		issuer := &certmanagerv1.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      issuerName,
				Namespace: certTestNamespace,
			},
			Spec: certmanagerv1.IssuerSpec{
				IssuerConfig: certmanagerv1.IssuerConfig{
					SelfSigned: &certmanagerv1.SelfSignedIssuer{},
				},
			},
		}

		_, err := certClient.CertmanagerV1().Issuers(certTestNamespace).Create(ctx, issuer, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create Issuer, retrying")
			return false, nil
		}

		logger.Info("created Issuer successfully")
		return true, nil
	})
}

func createCertificate(ctx context.Context, logger logr.Logger, certClient certmanagerclientset.Interface, namespace, certificate string) error {
	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		cert := &certmanagerv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      certificate,
				Namespace: namespace,
			},
			Spec: certmanagerv1.CertificateSpec{
				DNSNames: []string{
					"example.com",
				},
				SecretName: certSecretName,
				IssuerRef: cmmeta.ObjectReference{
					Name: issuerName,
				},
			},
		}

		_, err := certClient.CertmanagerV1().Certificates(namespace).Create(ctx, cert, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create Certificate, retrying")
			return false, nil
		}

		logger.Info("Created Certificate successfully")
		return true, nil
	})
}

func validateCertificate(ctx context.Context, logger logr.Logger, certClient certmanagerclientset.Interface, namespace, certificate string) error {
	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		cert, err := certClient.CertmanagerV1().Certificates(namespace).Get(ctx, certificate, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error getting secret: %v", err)
		}

		for _, condition := range cert.Status.Conditions {
			if condition.Type == certmanagerv1.CertificateConditionReady && condition.Status == cmmeta.ConditionTrue {
				logger.Info("certificate validated successfully")
				return true, nil
			}
		}

		logger.Info("certificate not valid yet")
		return false, nil
	})
}
