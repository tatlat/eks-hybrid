package addon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acmpca"
	"github.com/aws/aws-sdk-go-v2/service/acmpca/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	awspcav1beta1 "github.com/cert-manager/aws-privateca-issuer/pkg/api/v1beta1"
	awspcaclientset "github.com/cert-manager/aws-privateca-issuer/pkg/clientset/v1beta1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgo "k8s.io/client-go/kubernetes"

	ik8s "github.com/aws/eks-hybrid/internal/kubernetes"
)

const (
	awsPcaIssuerName     = "aws-pca-issuer"
	awsPcaCertName       = "aws-pca-test-cert"
	awsPcaCertSecretName = "aws-pca-cert-tls"
	defaultPollInterval  = 5 * time.Second
	pcaIssuerWaitTimeout = 2 * time.Minute
)

// PCAIssuerTest tests the AWS PCA Issuer functionality as a plugin for cert-manager
type PCAIssuerTest struct {
	Cluster            string
	Namespace          string
	K8S                clientgo.Interface
	EKSClient          *eks.Client
	CertClient         certmanagerclientset.Interface
	K8sPcaClient       awspcaclientset.Interface
	PCAClient          *acmpca.Client
	Region             string
	PCAArn             *string
	PodIdentityRoleArn *string
	Logger             logr.Logger
}

// Setup initializes the AWS PCA Issuer test by creating the Private CA
func (p *PCAIssuerTest) Setup(ctx context.Context) error {
	p.Logger.Info("Setting up AWS PCA Issuer test")

	// Create and activate a Private CA
	if err := p.setupPrivateCA(ctx); err != nil {
		return fmt.Errorf("failed to setup AWS Private CA: %v", err)
	}

	if err := p.setupPodIdentityForPCAIssuer(ctx); err != nil {
		return fmt.Errorf("failed to setup Pod Identity for AWS PCA Issuer: %v", err)
	}

	return nil
}

// Validate tests the AWS PCA Issuer by creating and validating certificates
func (p *PCAIssuerTest) Validate(ctx context.Context) error {
	p.Logger.Info("Validating AWS PCA Issuer")

	// Create AWS PCA Issuer resource
	if err := p.createAwsPcaIssuer(ctx); err != nil {
		return fmt.Errorf("failed to create AWS PCA Issuer: %v", err)
	}

	// Create certificate using AWS PCA Issuer
	if err := p.createAwsPcaCertificate(ctx); err != nil {
		return fmt.Errorf("failed to create certificate using AWS PCA Issuer: %v", err)
	}

	// Validate AWS PCA certificate
	if err := p.validateAwsPcaCertificate(ctx); err != nil {
		return fmt.Errorf("failed to validate AWS PCA certificate: %v", err)
	}

	p.Logger.Info("AWS PCA Issuer validation completed successfully")
	return nil
}

// Cleanup removes all the AWS PCA Issuer resources
func (p *PCAIssuerTest) Cleanup(ctx context.Context) error {
	p.Logger.Info("Cleaning up AWS PCA Issuer resources")

	// Delete AWS PCA certificate
	err := p.CertClient.CertmanagerV1().Certificates(p.Namespace).Delete(
		ctx, awsPcaCertName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		p.Logger.Error(err, "Failed to delete AWS PCA certificate")
	}

	// Delete AWS PCA Issuer
	err = p.K8sPcaClient.AWSPCAIssuers(p.Namespace).Delete(
		ctx, awsPcaIssuerName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		p.Logger.Error(err, "Failed to delete AWS PCA Issuer")
	}

	// Clean up Pod Identity if used
	if p.PodIdentityRoleArn != nil {
		if err := p.cleanupPodIdentityForPCAIssuer(ctx); err != nil {
			p.Logger.Error(err, "Failed to clean up Pod Identity for AWS PCA Issuer")
		}
	}

	// Clean up Private CA
	if p.PCAArn != nil {
		if err := p.cleanupPrivateCA(ctx); err != nil {
			p.Logger.Error(err, "Failed to clean up AWS Private CA")
		}
	}

	return nil
}

// Helper functions

func (p *PCAIssuerTest) setupPrivateCA(ctx context.Context) error {
	p.Logger.Info("Setting up AWS Private Certificate Authority")

	// Create a new Private CA
	pcaArn, err := p.createPCA(ctx)
	if err != nil {
		return err
	}
	p.PCAArn = pcaArn

	// Activate the Private CA
	if err := p.activatePCA(ctx, pcaArn); err != nil {
		return err
	}

	p.Logger.Info("AWS Private Certificate Authority created and activated", "ARN", *pcaArn)
	return nil
}

func (p *PCAIssuerTest) createPCA(ctx context.Context) (*string, error) {
	// Create the CA
	input := &acmpca.CreateCertificateAuthorityInput{
		CertificateAuthorityConfiguration: &types.CertificateAuthorityConfiguration{
			KeyAlgorithm:     types.KeyAlgorithmRsa2048,
			SigningAlgorithm: types.SigningAlgorithmSha256withrsa,
			Subject: &types.ASN1Subject{
				CommonName:         aws.String("Example CA"),
				Country:            aws.String("US"),
				Organization:       aws.String("Example Organization"),
				OrganizationalUnit: aws.String("IT Department"),
				State:              aws.String("Washington"),
				Locality:           aws.String("Seattle"),
			},
		},
		CertificateAuthorityType: types.CertificateAuthorityTypeRoot,
	}

	result, err := p.PCAClient.CreateCertificateAuthority(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create certificate authority: %v", err)
	}

	// For AWS resources, use standard polling since AWS SDK has built-in retry
	err = wait.PollUntilContextTimeout(ctx, defaultPollInterval, pcaIssuerWaitTimeout, true, func(ctx context.Context) (bool, error) {
		describeInput := &acmpca.DescribeCertificateAuthorityInput{
			CertificateAuthorityArn: result.CertificateAuthorityArn,
		}

		describeOutput, err := p.PCAClient.DescribeCertificateAuthority(ctx, describeInput)
		if err != nil {
			return false, fmt.Errorf("describe CA: %v", err)
		}

		status := describeOutput.CertificateAuthority.Status

		if status == types.CertificateAuthorityStatusPendingCertificate {
			return true, nil // Ready for next steps
		}

		if status != types.CertificateAuthorityStatusCreating {
			return false, fmt.Errorf("unexpected CA status: %s", status)
		}

		return false, nil // Continue waiting
	})

	if err != nil {
		return nil, fmt.Errorf("waiting for CA creation: %v", err)
	}

	return result.CertificateAuthorityArn, nil
}

func (p *PCAIssuerTest) activatePCA(ctx context.Context, arn *string) error {
	// Get the CSR
	csrOutput, err := p.PCAClient.GetCertificateAuthorityCsr(ctx, &acmpca.GetCertificateAuthorityCsrInput{
		CertificateAuthorityArn: arn,
	})
	if err != nil {
		return fmt.Errorf("get CSR: %v", err)
	}

	// Issue certificate for the CA
	certInput := &acmpca.IssueCertificateInput{
		CertificateAuthorityArn: arn,
		Csr:                     []byte(*csrOutput.Csr),
		SigningAlgorithm:        types.SigningAlgorithmSha256withrsa,
		TemplateArn:             aws.String("arn:aws:acm-pca:::template/RootCACertificate/V1"),
		Validity: &types.Validity{
			Type:  types.ValidityPeriodTypeDays,
			Value: aws.Int64(3650), // 10 years
		},
	}

	certOutput, err := p.PCAClient.IssueCertificate(ctx, certInput)
	if err != nil {
		return fmt.Errorf("issue certificate: %v", err)
	}

	// Wait for the certificate to be issued
	err = wait.PollUntilContextTimeout(ctx, defaultPollInterval, pcaIssuerWaitTimeout, true, func(ctx context.Context) (bool, error) {
		getCertInput := &acmpca.GetCertificateInput{
			CertificateAuthorityArn: arn,
			CertificateArn:          certOutput.CertificateArn,
		}

		_, err := p.PCAClient.GetCertificate(ctx, getCertInput)
		if err != nil {
			// If it's still processing, we continue waiting
			if strings.Contains(err.Error(), "RequestInProgressException") {
				return false, nil
			}
			return false, fmt.Errorf("get certificate: %v", err)
		}

		return true, nil // Certificate is issued
	})

	if err != nil {
		return fmt.Errorf("waiting for certificate issuance: %v", err)
	}

	// Get the issued certificate
	getCertOutput, err := p.PCAClient.GetCertificate(ctx, &acmpca.GetCertificateInput{
		CertificateAuthorityArn: arn,
		CertificateArn:          certOutput.CertificateArn,
	})
	if err != nil {
		return fmt.Errorf("get certificate: %v", err)
	}

	// Import the certificate back to the CA
	importInput := &acmpca.ImportCertificateAuthorityCertificateInput{
		CertificateAuthorityArn: arn,
		Certificate:             []byte(*getCertOutput.Certificate),
	}

	_, err = p.PCAClient.ImportCertificateAuthorityCertificate(ctx, importInput)
	if err != nil {
		return fmt.Errorf("import certificate: %v", err)
	}

	// Update the CA status to ACTIVE
	updateInput := &acmpca.UpdateCertificateAuthorityInput{
		CertificateAuthorityArn: arn,
		Status:                  types.CertificateAuthorityStatusActive,
	}

	_, err = p.PCAClient.UpdateCertificateAuthority(ctx, updateInput)
	if err != nil {
		return fmt.Errorf("activate CA: %v", err)
	}

	return nil
}

func (p *PCAIssuerTest) setupPodIdentityForPCAIssuer(ctx context.Context) error {
	p.Logger.Info("Setting up Pod Identity for AWS PCA Issuer")

	// Create service account for AWS PCA Issuer
	svcAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      awsPcaIssuerName,
			Namespace: p.Namespace,
		},
	}

	_, err := p.K8S.CoreV1().ServiceAccounts(p.Namespace).Create(ctx, svcAccount, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create service account: %v", err)
	}

	// Create Pod Identity Association
	createAssociationInput := &eks.CreatePodIdentityAssociationInput{
		ClusterName:    aws.String(p.Cluster),
		Namespace:      aws.String(p.Namespace),
		RoleArn:        p.PodIdentityRoleArn,
		ServiceAccount: aws.String(svcAccount.Name),
	}

	createAssociationOutput, err := p.EKSClient.CreatePodIdentityAssociation(ctx, createAssociationInput)
	if err != nil {
		return fmt.Errorf("failed to create Pod Identity Association: %v", err)
	}

	p.Logger.Info("Created Pod Identity Association", "associationID", *createAssociationOutput.Association.AssociationId)
	return nil
}

func (p *PCAIssuerTest) createAwsPcaIssuer(ctx context.Context) error {
	p.Logger.Info("Creating AWS PCA Issuer resource")

	awsPcaIssuer := &awspcav1beta1.AWSPCAIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      awsPcaIssuerName,
			Namespace: p.Namespace,
		},
		Spec: awspcav1beta1.AWSPCAIssuerSpec{
			Arn:    *p.PCAArn,
			Region: p.Region,
		},
	}

	// Direct create without retry
	_, err := p.K8sPcaClient.AWSPCAIssuers(p.Namespace).Create(ctx, awsPcaIssuer, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	p.Logger.Info("Created or confirmed existing AWS PCA Issuer")
	return nil
}

func (p *PCAIssuerTest) createAwsPcaCertificate(ctx context.Context) error {
	p.Logger.Info("Creating certificate using AWS PCA Issuer")

	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      awsPcaCertName,
			Namespace: p.Namespace,
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

	_, err := p.CertClient.CertmanagerV1().Certificates(p.Namespace).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	p.Logger.Info("Created or confirmed existing certificate using AWS PCA Issuer")
	return nil
}

func (p *PCAIssuerTest) validateAwsPcaCertificate(ctx context.Context) error {
	p.Logger.Info("Validating AWS PCA certificate")

	_, err := ik8s.GetAndWait(
		ctx,
		pcaIssuerWaitTimeout,
		p.CertClient.CertmanagerV1().Certificates(p.Namespace),
		awsPcaCertName,
		func(cert *certmanagerv1.Certificate) bool {
			for _, condition := range cert.Status.Conditions {
				if condition.Type == certmanagerv1.CertificateConditionReady && condition.Status == cmmeta.ConditionTrue {
					p.Logger.Info("AWS PCA certificate validated successfully")
					return true
				}
			}
			p.Logger.Info("AWS PCA certificate not valid yet")
			return false
		},
	)

	if err != nil {
		return fmt.Errorf("error validating AWS PCA certificate: %v", err)
	}

	return nil
}

func (p *PCAIssuerTest) cleanupPodIdentityForPCAIssuer(ctx context.Context) error {
	p.Logger.Info("Cleaning up Pod Identity for AWS PCA Issuer")

	// List Pod Identity Associations
	listAssociationsInput := &eks.ListPodIdentityAssociationsInput{
		ClusterName:    aws.String(p.Cluster),
		Namespace:      aws.String(p.Namespace),
		ServiceAccount: aws.String(awsPcaIssuerName),
	}

	listAssociationsOutput, err := p.EKSClient.ListPodIdentityAssociations(ctx, listAssociationsInput)
	if err != nil {
		return fmt.Errorf("failed to list Pod Identity Associations: %v", err)
	}

	// Delete each found association
	for _, association := range listAssociationsOutput.Associations {
		if association.AssociationId != nil {
			p.Logger.Info("Deleting Pod Identity Association", "associationID", *association.AssociationId)
			deleteAssociationInput := &eks.DeletePodIdentityAssociationInput{
				ClusterName:   aws.String(p.Cluster),
				AssociationId: association.AssociationId,
			}

			_, err := p.EKSClient.DeletePodIdentityAssociation(ctx, deleteAssociationInput)
			if err != nil {
				return fmt.Errorf("failed to delete Pod Identity Association: %v", err)
			}
		}
	}

	// Delete service account
	err = p.K8S.CoreV1().ServiceAccounts(p.Namespace).Delete(ctx, awsPcaIssuerName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		p.Logger.Error(err, "Failed to delete service account")
	}

	return nil
}

func (p *PCAIssuerTest) cleanupPrivateCA(ctx context.Context) error {
	p.Logger.Info("Cleaning up AWS Private Certificate Authority")

	if p.PCAArn == nil {
		return nil
	}

	// Disable the CA first
	_, err := p.PCAClient.UpdateCertificateAuthority(ctx, &acmpca.UpdateCertificateAuthorityInput{
		CertificateAuthorityArn: p.PCAArn,
		Status:                  types.CertificateAuthorityStatusDisabled,
	})
	if err != nil {
		return fmt.Errorf("failed to disable CA: %v", err)
	}

	// Wait for the CA to be disabled
	err = wait.PollUntilContextTimeout(ctx, defaultPollInterval, pcaIssuerWaitTimeout, true, func(ctx context.Context) (bool, error) {
		describeInput := &acmpca.DescribeCertificateAuthorityInput{
			CertificateAuthorityArn: p.PCAArn,
		}

		describeOutput, err := p.PCAClient.DescribeCertificateAuthority(ctx, describeInput)
		if err != nil {
			return false, fmt.Errorf("describe CA: %v", err)
		}

		status := describeOutput.CertificateAuthority.Status
		return status == types.CertificateAuthorityStatusDisabled, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for CA to be disabled: %v", err)
	}

	// Delete the CA
	_, err = p.PCAClient.DeleteCertificateAuthority(ctx, &acmpca.DeleteCertificateAuthorityInput{
		CertificateAuthorityArn: p.PCAArn,
	})
	if err != nil {
		return fmt.Errorf("failed to delete CA: %v", err)
	}

	p.Logger.Info("AWS Private Certificate Authority deleted successfully")
	return nil
}
