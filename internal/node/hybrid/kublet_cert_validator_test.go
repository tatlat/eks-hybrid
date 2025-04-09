package hybrid

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/kubelet"
)

func TestValidateKubeletCert(t *testing.T) {
	g := NewGomegaWithT(t)
	logger := zap.NewNop()

	caBytes, ca, caKey := generateCA(g)
	_, wrongCA, wrongCAKey := generateCA(g)

	tests := []struct {
		name          string
		cert          []byte
		ca            []byte
		skipPhases    []string
		expectedError string
		setup         func(tempDir string) error
	}{
		{
			name:          "no existing cert",
			expectedError: "",
		},
		{
			name:          "valid existing cert",
			cert:          generateKubeletCert(g, ca, caKey, time.Now(), time.Now().AddDate(10, 0, 0)),
			ca:            caBytes,
			expectedError: "",
		},
		{
			name:          "expired certificate",
			cert:          generateKubeletCert(g, ca, caKey, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, -1)),
			ca:            caBytes,
			expectedError: "",
		},
		{
			name:          "wrong CA",
			cert:          generateKubeletCert(g, wrongCA, wrongCAKey, time.Now(), time.Now().AddDate(10, 0, 0)),
			ca:            caBytes,
			expectedError: "kubelet certificate is not valid for the current cluster",
		},
		{
			name:          "skip validation",
			cert:          generateKubeletCert(g, wrongCA, wrongCAKey, time.Now(), time.Now().AddDate(10, 0, 0)),
			ca:            caBytes,
			skipPhases:    []string{kubeletCertValidation},
			expectedError: "",
		},
		{
			name:          "stat error",
			expectedError: "reading kubelet certificate",
			setup: func(tempDir string) error {
				// Create a directory with the same name as the cert file to cause a stat error
				certPath := filepath.Join(tempDir, kubelet.KubeletCurrentCertPath)
				return os.MkdirAll(certPath, 0o755)
			},
		},
		{
			name:          "invalid cert format",
			cert:          []byte("invalid pem data"),
			ca:            caBytes,
			expectedError: "parsing kubelet certificate",
		},
		{
			name:          "invalid CA format",
			cert:          generateKubeletCert(g, ca, caKey, time.Now(), time.Now().AddDate(10, 0, 0)),
			ca:            []byte("invalid ca data"),
			expectedError: "parsing cluster CA certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			// Create a temporary directory for test files
			tempDir, err := os.MkdirTemp("", "hybrid-test-*")
			g.Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			// Create the directory structure for the kubelet cert
			certDir := filepath.Join(tempDir, filepath.Dir(kubelet.KubeletCurrentCertPath))
			g.Expect(os.MkdirAll(certDir, 0o755)).To(Succeed())

			if tt.setup != nil {
				g.Expect(tt.setup(tempDir)).To(Succeed())
			}

			certPath := filepath.Join(tempDir, kubelet.KubeletCurrentCertPath)
			if tt.cert != nil {
				err := os.WriteFile(certPath, tt.cert, 0o600)
				g.Expect(err).NotTo(HaveOccurred())
			}

			nodeConfig := &api.NodeConfig{
				Spec: api.NodeConfigSpec{
					Cluster: api.ClusterDetails{
						CertificateAuthority: tt.ca,
					},
				},
			}

			provider, err := NewHybridNodeProvider(nodeConfig, tt.skipPhases, logger, WithInstallRoot(tempDir))
			g.Expect(err).NotTo(HaveOccurred())

			err = provider.Validate()
			if tt.expectedError == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(MatchError(ContainSubstring(tt.expectedError)))
			}
		})
	}
}

func generateCA(g *WithT) ([]byte, *x509.Certificate, *ecdsa.PrivateKey) {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2025),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			CommonName:   "test-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	g.Expect(err).NotTo(HaveOccurred())

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	g.Expect(err).NotTo(HaveOccurred())

	certPEM := new(bytes.Buffer)
	g.Expect(pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})).NotTo(HaveOccurred())

	return certPEM.Bytes(), cert, privateKey
}

func generateKubeletCert(g *WithT, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey, validFrom, validTo time.Time) []byte {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2025),
		Subject: pkix.Name{
			Organization: []string{"Test Kubelet"},
			CommonName:   "test-kubelet",
		},
		NotBefore:             validFrom,
		NotAfter:              validTo,
		IsCA:                  false,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	g.Expect(err).NotTo(HaveOccurred())

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, issuer, &privateKey.PublicKey, issuerKey)
	g.Expect(err).NotTo(HaveOccurred())

	certPEM := new(bytes.Buffer)
	g.Expect(pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})).NotTo(HaveOccurred())

	return certPEM.Bytes()
}
