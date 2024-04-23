package iamrolesanywhere_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/iamrolesanywhere"
)

func TestEnsureAWSConfig_Write(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aws-config")

	expect, err := os.ReadFile("./testdata/aws-config")
	if err != nil {
		t.Fatal(err)
	}

	cfg := iamrolesanywhere.AWSConfig{
		TrustAnchorARN: "trust-anchor",
		ProfileARN:     "profile",
		RoleARN:        "role",
		Region:         "region",
		ConfigPath:     path,
	}

	err = iamrolesanywhere.EnsureAWSConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if stat.Mode() != 0644 {
		t.Fatalf("Expected mode: %v; Received: %v", 0644, stat.Mode())
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

	if err := os.WriteFile(path, expect, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := iamrolesanywhere.AWSConfig{
		TrustAnchorARN: "trust-anchor",
		ProfileARN:     "profile",
		RoleARN:        "role",
		Region:         "region",
		ConfigPath:     path,
	}

	err = iamrolesanywhere.EnsureAWSConfig(cfg)
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

	if err := os.WriteFile(path, []byte("incorrect data"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := iamrolesanywhere.AWSConfig{
		TrustAnchorARN: "trust-anchor",
		ProfileARN:     "profile",
		RoleARN:        "role",
		Region:         "region",
		ConfigPath:     path,
	}

	err := iamrolesanywhere.EnsureAWSConfig(cfg)
	if err == nil {
		t.Fatal("Expeted error, received nil")
	}
}
