package addon

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"

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
	certName                  = "my-cert-request"
	certTestNamespace         = "default"
	issuerName                = "selfsigned-issuer"
)

type CertificateData struct {
	CSR        []byte
	PrivateKey []byte
}

type CertManagerTest struct {
	Cluster    string
	addon      Addon
	K8S        clientgo.Interface
	EKSClient  *eks.Client
	K8SConfig  *rest.Config
	Logger     logr.Logger
	CertClient certmanagerclientset.Interface
}

func (c CertManagerTest) Run(ctx context.Context) error {
	c.addon = Addon{
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

func (c CertManagerTest) Create(ctx context.Context) error {
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

	c.Logger.Info("All cert-manager components are ready")
	return nil
}

func (c CertManagerTest) Validate(ctx context.Context) error {
	c.Logger.Info("Starting cert-manager validation")

	// Step 1: Create self-signed issuer with retries
	if err := createSelfSignedIssuer(ctx, c.Logger, c.CertClient); err != nil {
		return fmt.Errorf("failed to create self-signed issuer: %v", err)
	}

	// Step 2: Generate CSR and create certificate request
	certData, err := generateCSR("example.com", []string{"example.com", "www.example.com"})
	if err != nil {
		return fmt.Errorf("failed to generate CSR: %v", err)
	}

	if err = createCertificateRequest(ctx, c.Logger, c.CertClient, certTestNamespace, certData); err != nil {
		return fmt.Errorf("failed to create certificate request: %v", err)
	}

	// Step 3: check certificate secret
	if err = waitForCertificateSecret(ctx, c.Logger, c.K8S, certTestNamespace, certName, certData); err != nil {
		return fmt.Errorf("Failed to validate certificate secret: %v", err)
	}

	c.Logger.Info("Cert-manager validation completed")
	return nil
}

func (c CertManagerTest) CollectLogs(ctx context.Context) error {
	// Collect logs from all cert-manager components
	components := []string{certManagerName, certManagerWebhookName, certManagerCainjectorName}
	for _, component := range components {
		if err := c.addon.FetchLogs(ctx, c.K8S, c.Logger, components, tailLines); err != nil {
			c.Logger.Error(err, "failed to collect logs for component", "component", component)
		}
	}

	return nil
}

func (c CertManagerTest) Delete(ctx context.Context) error {
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

		err = c.CertClient.CertmanagerV1().ClusterIssuers().Delete(
			ctx, issuerName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			c.Logger.Error(err, "failed to delete ClusterIssuer, retrying", "name", issuerName)
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
		issuer := &certmanagerv1.ClusterIssuer{
			ObjectMeta: metav1.ObjectMeta{
				Name: issuerName,
			},
			Spec: certmanagerv1.IssuerSpec{
				IssuerConfig: certmanagerv1.IssuerConfig{
					SelfSigned: &certmanagerv1.SelfSignedIssuer{},
				},
			},
		}

		_, err := certClient.CertmanagerV1().ClusterIssuers().Create(ctx, issuer, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create ClusterIssuer, retrying")
			return false, nil
		}

		logger.Info("created ClusterIssuer successfully")
		return true, nil
	})
}

func generateCSR(commonName string, dnsNames []string) (*CertificateData, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// certificate request template
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames: dnsNames,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSR: %v", err)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return &CertificateData{
		CSR:        csrPEM,
		PrivateKey: privateKeyPEM,
	}, nil
}

func createCertificateRequest(ctx context.Context, logger logr.Logger, certClient certmanagerclientset.Interface, namespace string, certData *CertificateData) error {
	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		cr := &certmanagerv1.CertificateRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      certName,
				Namespace: namespace,
			},
			Spec: certmanagerv1.CertificateRequestSpec{
				Request: certData.CSR,
				IsCA:    false,
				Usages: []certmanagerv1.KeyUsage{
					certmanagerv1.UsageDigitalSignature,
					certmanagerv1.UsageKeyEncipherment,
					certmanagerv1.UsageServerAuth,
				},
				Duration: &metav1.Duration{
					Duration: 24 * time.Hour,
				},
				IssuerRef: cmmeta.ObjectReference{
					Name:  issuerName,
					Kind:  "ClusterIssuer",
					Group: "cert-manager.io",
				},
			},
		}

		_, err := certClient.CertmanagerV1().CertificateRequests(namespace).Create(ctx, cr, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			logger.Error(err, "failed to create CertificateRequest, retrying")
			return false, nil
		}

		logger.Info("Created CertificateRequest successfully")
		return true, nil
	})
}

func validateCertificateAndKey(certPEM, keyPEM, originalKeyPEM []byte) error {
	// Parse the certificate
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %v", err)
	}

	// Parse the secret's private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to parse private key PEM")
	}

	// Parse the original private key
	originalKeyBlock, _ := pem.Decode(originalKeyPEM)
	if originalKeyBlock == nil {
		return fmt.Errorf("failed to parse original private key PEM")
	}

	// Compare the two private keys
	if string(keyBlock.Bytes) != string(originalKeyBlock.Bytes) {
		return fmt.Errorf("private key in secret does not match original private key")
	}

	// Verify certificate validity
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not valid yet")
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}

	return nil
}

func waitForCertificateSecret(ctx context.Context, logger logr.Logger, k8sClient clientgo.Interface, namespace, secretName string, certData *CertificateData) error {
	return wait.ExponentialBackoffWithContext(ctx, retryBackoff, func(ctx context.Context) (bool, error) {
		secret, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error getting secret: %v", err)
		}

		if secret.Type != v1.SecretTypeTLS {
			logger.Info("Secret is not of type TLS, waiting for correct type...")
			return false, nil
		}

		certPEM, ok := secret.Data["tls.crt"]
		if !ok {
			logger.Info("tls.crt not found in secret, waiting...")
			return false, nil
		}

		keyPEM, ok := secret.Data["tls.key"]
		if !ok {
			logger.Info("tls.key not found in secret, waiting...")
			return false, nil
		}

		if err := validateCertificateAndKey(certPEM, keyPEM, certData.PrivateKey); err != nil {
			logger.Info("Certificate validation failed: %v, waiting...", err)
			return false, nil
		}

		logger.Info("Certificate secret validated successfully")
		return true, nil
	})
}
