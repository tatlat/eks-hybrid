package iamrolesanywhere_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
)

func TestEnsureAWSConfig_Write(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aws-config")

	expect, err := os.ReadFile("./testdata/aws-config")
	if err != nil {
		t.Fatal(err)
	}

	cfg := iamrolesanywhere.AWSConfig{
		TrustAnchorARN:       "trust-anchor",
		ProfileARN:           "profile",
		RoleARN:              "role",
		Region:               "region",
		NodeName:             "test01",
		ConfigPath:           path,
		SigningHelperBinPath: "/random/path",
		CertificatePath:      "/etc/certificates/iam/pki/my-server.crt",
		PrivateKeyPath:       "/etc/certificates/iam/pki/my-server.key",
	}

	err = iamrolesanywhere.WriteAWSConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if stat.Mode() != 0o644 {
		t.Fatalf("Expected mode: %v; Received: %v", 0o644, stat.Mode())
	}

	received, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(expect, received) {
		t.Fatalf("Found unexpected content.\nReceived:\n%s\n\nExpect:\n%s\n", received, expect)
	}
}

func TestEnsureAWSConfig_ExistsSameContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aws-config")

	expect, err := os.ReadFile("./testdata/aws-config")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, expect, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := iamrolesanywhere.AWSConfig{
		TrustAnchorARN:       "trust-anchor",
		ProfileARN:           "profile",
		RoleARN:              "role",
		NodeName:             "test01",
		Region:               "region",
		ConfigPath:           path,
		SigningHelperBinPath: "/random/path",
		CertificatePath:      "/etc/certificates/iam/pki/my-server.crt",
		PrivateKeyPath:       "/etc/certificates/iam/pki/my-server.key",
	}

	err = iamrolesanywhere.WriteAWSConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	received, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(expect, received) {
		t.Fatalf("Found unexpected content.\nReceived:\n%s\n\nExpect:\n%s\n", string(received), expect)
	}
}

func TestEnsureAWSConfig_ExistsDifferentContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aws-config")

	if err := os.WriteFile(path, []byte("incorrect data"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := iamrolesanywhere.AWSConfig{
		TrustAnchorARN:  "trust-anchor",
		ProfileARN:      "profile",
		RoleARN:         "role",
		NodeName:        "test01",
		Region:          "region",
		ConfigPath:      path,
		CertificatePath: "/etc/certificates/iam/pki/my-server.crt",
		PrivateKeyPath:  "/etc/certificates/iam/pki/my-server.key",
	}

	err := iamrolesanywhere.WriteAWSConfig(cfg)
	if err == nil {
		t.Fatal("Expeted error, received nil")
	}
}

func TestWriteAWSConfigValidation(t *testing.T) {
	testCases := []struct {
		name    string
		config  iamrolesanywhere.AWSConfig
		wantErr string
	}{
		{
			name: "empty cert",
			config: iamrolesanywhere.AWSConfig{
				TrustAnchorARN:       "trust-anchor",
				SigningHelperBinPath: "/random/path",
				ProfileARN:           "profile",
				RoleARN:              "role",
				Region:               "region",
				NodeName:             "test01",
				PrivateKeyPath:       "/etc/iam/pki/server.key",
			},
			wantErr: "CertificatePath cannot be empty",
		},
		{
			name: "key cert",
			config: iamrolesanywhere.AWSConfig{
				TrustAnchorARN:       "trust-anchor",
				SigningHelperBinPath: "/random/path",
				ProfileARN:           "profile",
				RoleARN:              "role",
				Region:               "region",
				NodeName:             "test01",
				CertificatePath:      "/etc/iam/pki/server.crt",
			},
			wantErr: "PrivateKeyPath cannot be empty",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(
				iamrolesanywhere.WriteAWSConfig(tc.config),
			).To(MatchError(tc.wantErr))
		})
	}
}
