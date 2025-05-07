package addon

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/eks-hybrid/test/e2e/kubernetes"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/go-logr/logr"
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
)

type CertificateData struct {
	CSR        []byte
	PrivateKey []byte
}

type CertManagerTest struct {
	Cluster    string
	addon      *Addon
	K8S        clientgo.Interface
	EKSClient  *eks.Client
	K8SConfig  *rest.Config
	Logger     logr.Logger
	CertClient certmanagerclientset.Interface
}

func (c *CertManagerTest) Run(ctx context.Context) error {
	c.addon = &Addon{
		Cluster:   c.Cluster,
		Namespace: certManagerNamespace,
		Name:      certManagerName,
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

	if _, err := c.K8S.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: certTestNamespace,
		},
	}, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	c.Logger.Info("All cert-manager components are ready")
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

	c.Logger.Info("Cert-manager validation completed")
	return nil
}

func (c *CertManagerTest) CollectLogs(ctx context.Context) error {
	// Collect logs from all cert-manager components
	if err := c.addon.FetchLogs(ctx, c.K8S, c.Logger); err != nil {
		c.Logger.Error(err, "failed to collect logs for cert-manager")
	}

	return nil
}

func (c *CertManagerTest) Delete(ctx context.Context) error {
	c.Logger.Info("starting cleanup of all test resources")

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
			c.Logger.Error(err, "failed to delete test mamespace, retrying", "name", certTestNamespace)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		c.Logger.Error(err, "failed to delete cert-manager k8s resources")
	}

	if err := c.addon.Delete(ctx, c.EKSClient, c.Logger); err != nil {
		return err
	}

	c.Logger.Info("successfully cleaned up all cert-manager test resources")
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
