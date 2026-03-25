package aws

import (
	"testing"
)

func TestGetManifestURL(t *testing.T) {
	// Set a default embedded manifest URL for testing
	originalManifestUrl := manifestUrl
	manifestUrl = "https://hybrid-assets.eks.amazonaws.com/manifest.yaml"
	defer func() { manifestUrl = originalManifestUrl }()

	tests := []struct {
		name     string
		region   string
		expected string
	}{
		{
			name:     "Empty region uses default embedded URL",
			region:   "",
			expected: "https://hybrid-assets.eks.amazonaws.com/manifest.yaml",
		},
		{
			name:     "Standard AWS region uses default embedded URL",
			region:   "us-east-1",
			expected: "https://hybrid-assets.eks.amazonaws.com/manifest.yaml",
		},
		{
			name:     "Another standard AWS region uses default embedded URL",
			region:   "eu-west-1",
			expected: "https://hybrid-assets.eks.amazonaws.com/manifest.yaml",
		},
		{
			name:     "AWS China region uses China-specific URL",
			region:   "cn-north-1",
			expected: "https://eks-hybrid-assets.awsstatic.cn/manifest.yaml",
		},
		{
			name:     "AWS China northwest region uses China-specific URL",
			region:   "cn-northwest-1",
			expected: "https://eks-hybrid-assets.awsstatic.cn/manifest.yaml",
		},
		{
			name:     "AWS GovCloud region uses default embedded URL",
			region:   "us-gov-west-1",
			expected: "https://hybrid-assets.eks.amazonaws.com/manifest.yaml",
		},
		{
			name:     "AWS ISO region uses default embedded URL",
			region:   "us-iso-east-1",
			expected: "https://hybrid-assets.eks.amazonaws.com/manifest.yaml",
		},
		{
			name:     "AWS ISOB region uses default embedded URL",
			region:   "us-isob-east-1",
			expected: "https://hybrid-assets.eks.amazonaws.com/manifest.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getManifestURL(tt.region)
			if result != tt.expected {
				t.Errorf("getManifestURL(%q) = %q, want %q", tt.region, result, tt.expected)
			}
		})
	}
}

func TestGetManifestURLWithDifferentEmbeddedDefault(t *testing.T) {
	// Test that the function respects the embedded manifest URL
	originalManifestUrl := manifestUrl
	manifestUrl = "https://custom-host.example.com/manifest.yaml"
	defer func() { manifestUrl = originalManifestUrl }()

	tests := []struct {
		name     string
		region   string
		expected string
	}{
		{
			name:     "Standard region uses custom embedded URL",
			region:   "us-west-2",
			expected: "https://custom-host.example.com/manifest.yaml",
		},
		{
			name:     "China region constructs URL based on partition",
			region:   "cn-north-1",
			expected: "https://eks-hybrid-assets.awsstatic.cn/manifest.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getManifestURL(tt.region)
			if result != tt.expected {
				t.Errorf("getManifestURL(%q) = %q, want %q", tt.region, result, tt.expected)
			}
		})
	}
}
