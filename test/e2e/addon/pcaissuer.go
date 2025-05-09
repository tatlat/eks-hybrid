package addon

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	awspcaapi "github.com/cert-manager/aws-privateca-issuer/pkg/api/v1beta1"
	pcaclientset "github.com/cert-manager/aws-privateca-issuer/pkg/clientset/v1beta1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	pcaIssuerNamespace     = "aws-pca-issuer"
	pcaIssuerName          = "aws-privateca-issuer"
	pcaIssuerTestNamespace = "pca-test"
	pcaIssuerCRDName       = "awspcaissuer"
	pcaIssuerInstanceName  = "test-pca-issuer"
	pcaCertName            = "pca-cert"
	pcaCertSecretName      = "pca-cert-tls"

	// Maximum time to wait for PCA operations
	pcaTimeout = 5 * time.Minute
)

// PCAIssuerTest tests the AWS PCA Issuer addon
type PCAIssuerTest struct {
	Cluster    string
	addon      *Addon
	K8S        clientgo.Interface
	EKSClient  *eks.Client
	K8SConfig  *rest.Config
	Logger     logr.Logger
	CertClient certmanagerclientset.Interface
	PCAClient  pcaclientset.Interface
}

// Run executes the full test sequence
func (p *PCAIssuerTest) Run(ctx context.Context) error {
	p.addon = &Addon{
		Cluster:   p.Cluster,
		Namespace: pcaIssuerNamespace,
		Name:      pcaIssuerName,
	}

	pcaClient, err := pcaclientset.NewForConfig(p.K8SConfig)
	if err != nil {
		return err
	}

	p.PCAClient = pcaClient

	if err := p.Create(ctx); err != nil {
		return err
	}

	if err := p.Validate(ctx); err != nil {
		return err
	}

	return nil
}

// Create installs the AWS PCA Issuer addon
func (p *PCAIssuerTest) Create(ctx context.Context) error {
	p.Logger.Info("Starting AWS PCA Issuer addon installation")

	// Install the AWS PCA Issuer addon
	if err := p.addon.CreateAddon(ctx, p.EKSClient, p.K8S, p.Logger); err != nil {
		return err
	}

	// Wait for the AWS PCA Issuer deployment to be ready
	if err := kubernetes.WaitForDeploymentReady(ctx, p.Logger, p.K8S, pcaIssuerNamespace, pcaIssuerName); err != nil {
		return err
	}

	// Create test namespace
	if _, err := p.K8S.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: pcaIssuerTestNamespace,
		},
	}, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	p.Logger.Info("AWS PCA Issuer addon installed successfully")
	return nil
}

// Validate checks if the AWS PCA Issuer is working correctly
func (p *PCAIssuerTest) Validate(ctx context.Context) error {
	p.Logger.Info("Starting AWS PCA Issuer validation")

	// Step 1: Create AWS PCA Issuer instance with retries
	if err := p.createAWSPCAIssuer(ctx); err != nil {
		return fmt.Errorf("failed to create AWS PCA Issuer instance: %v", err)
	}

	// Step 2: Create certificate using AWS PCA Issuer
	if err := p.createCertificate(ctx); err != nil {
		return fmt.Errorf("failed to create certificate using AWS PCA Issuer: %v", err)
	}

	// Step 3: Validate the certificate was issued
	if err := p.validateCertificate(ctx); err != nil {
		return fmt.Errorf("failed to validate certificate: %v", err)
	}

	p.Logger.Info("AWS PCA Issuer validation completed successfully")
	return nil
}

// createAWSPCAIssuer creates an AWS PCA Issuer instance
func (p *PCAIssuerTest) createAWSPCAIssuer(ctx context.Context) error {
	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		// Get the aws-privateca-issuer ServiceAccount token
		sa, err := p.K8S.CoreV1().ServiceAccounts(pcaIssuerNamespace).Get(ctx, pcaIssuerName, metav1.GetOptions{})
		if err != nil {
			p.Logger.Error(err, "Failed to get ServiceAccount", "name", pcaIssuerName)
			return false, nil
		}

		p.Logger.Info("Found ServiceAccount for AWS PCA Issuer", "name", sa.Name)

		// Create AWSPCAIssuer CR - using EKS IRSA and role with PCA permissions
		issuer := &awspcaapi.AWSPCAIssuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pcaIssuerInstanceName,
				Namespace: pcaIssuerTestNamespace,
			},
			Spec: awspcaapi.AWSPCAIssuerSpec{
				Region:    "us-west-2", // Configurable region
				SecretRef: awspcaapi.AWSCredentialsSecretReference{
					// Using IAM roles for service accounts (IRSA)
					// The controller will use the pod's IAM role
				},
				Arn: "arn:aws:acm-pca:us-west-2:123456789012:certificate-authority/example-ca-id", // This should be configured appropriately
			},
		}

		_, err = p.PCAClient.AWSPCAIssuers(pcaIssuerTestNamespace).Create(ctx, issuer, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			p.Logger.Error(err, "Failed to create AWSPCAIssuer, retrying")
			return false, nil
		}

		p.Logger.Info("Created AWSPCAIssuer successfully", "name", pcaIssuerInstanceName)
		return true, nil
	})
}

// createCertificate creates a certificate using the AWS PCA Issuer
func (p *PCAIssuerTest) createCertificate(ctx context.Context) error {
	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		cert := &certmanagerv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pcaCertName,
				Namespace: pcaIssuerTestNamespace,
			},
			Spec: certmanagerv1.CertificateSpec{
				CommonName: "example.com",
				DNSNames: []string{
					"example.com",
					"www.example.com",
				},
				SecretName: pcaCertSecretName,
				IssuerRef: cmmeta.ObjectReference{
					Group: "awspca.cert-manager.io",
					Kind:  "AWSPCAIssuer",
					Name:  pcaIssuerInstanceName,
				},
				// Additional fields can be added as needed for AWS PCA
				Duration:    &metav1.Duration{Duration: 24 * time.Hour},
				RenewBefore: &metav1.Duration{Duration: 1 * time.Hour},
			},
		}

		_, err := p.CertClient.CertmanagerV1().Certificates(pcaIssuerTestNamespace).Create(ctx, cert, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			p.Logger.Error(err, "Failed to create Certificate, retrying")
			return false, nil
		}

		p.Logger.Info("Created Certificate successfully", "name", pcaCertName)
		return true, nil
	})
}

// validateCertificate checks if the certificate is issued successfully
func (p *PCAIssuerTest) validateCertificate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, pcaTimeout)
	defer cancel()

	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		cert, err := p.CertClient.CertmanagerV1().Certificates(pcaIssuerTestNamespace).Get(ctx, pcaCertName, metav1.GetOptions{})
		if err != nil {
			p.Logger.Error(err, "Failed to get Certificate")
			return false, nil
		}

		// Check if certificate is ready
		for _, condition := range cert.Status.Conditions {
			p.Logger.Info("Certificate condition", "type", condition.Type, "status", condition.Status, "reason", condition.Reason)

			if condition.Type == certmanagerv1.CertificateConditionReady &&
				condition.Status == cmmeta.ConditionTrue {
				// Verify the secret exists
				_, err := p.K8S.CoreV1().Secrets(pcaIssuerTestNamespace).Get(ctx, pcaCertSecretName, metav1.GetOptions{})
				if err != nil {
					p.Logger.Error(err, "Certificate secret not found")
					return false, nil
				}

				p.Logger.Info("Certificate successfully issued and stored in secret",
					"certificate", pcaCertName,
					"secret", pcaCertSecretName)
				return true, nil
			}
		}

		p.Logger.Info("Certificate not ready yet", "name", pcaCertName)
		return false, nil
	})
}

// CollectLogs gathers logs for debugging
func (p *PCAIssuerTest) CollectLogs(ctx context.Context) error {
	return p.addon.FetchLogs(ctx, p.K8S, p.Logger)
}

// Delete removes the addon and test resources
func (p *PCAIssuerTest) Delete(ctx context.Context) error {
	p.Logger.Info("Starting cleanup of AWS PCA Issuer test resources")

	// Clean up test resources
	err := wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		// Delete Certificate
		if err := p.CertClient.CertmanagerV1().Certificates(pcaIssuerTestNamespace).Delete(
			ctx, pcaCertName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			p.Logger.Error(err, "Failed to delete Certificate, retrying")
			return false, nil
		}

		// Delete Certificate Secret
		if err := p.K8S.CoreV1().Secrets(pcaIssuerTestNamespace).Delete(
			ctx, pcaCertSecretName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			p.Logger.Error(err, "Failed to delete Certificate Secret, retrying")
			return false, nil
		}

		// Delete AWSPCAIssuer
		if err := p.PCAClient.AWSPCAIssuers(pcaIssuerTestNamespace).Delete(
			ctx, pcaIssuerInstanceName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			p.Logger.Error(err, "Failed to delete AWSPCAIssuer, retrying")
			return false, nil
		}

		// Delete test namespace
		if err := p.K8S.CoreV1().Namespaces().Delete(
			ctx, pcaIssuerTestNamespace, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			p.Logger.Error(err, "Failed to delete test namespace, retrying")
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		p.Logger.Error(err, "Failed to clean up some test resources")
	}

	// Delete the addon
	if err := p.addon.Delete(ctx, p.EKSClient, p.Logger); err != nil {
		return err
	}

	p.Logger.Info("Successfully cleaned up all AWS PCA Issuer test resources")
	return nil
}
