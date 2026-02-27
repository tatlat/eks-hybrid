package e2e

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type TestConfig struct {
	ClusterName     string `yaml:"clusterName"`
	ClusterRegion   string `yaml:"clusterRegion"`
	NodeadmUrlAMD   string `yaml:"nodeadmUrlAMD"`
	NodeadmUrlARM   string `yaml:"nodeadmUrlARM"`
	SetRootPassword bool   `yaml:"setRootPassword"`
	NodeK8sVersion  string `yaml:"nodeK8SVersion"`
	LogsBucket      string `yaml:"logsBucket"`
	Endpoint        string `yaml:"endpoint"`
	// ArtifactsFolder is the local path where the test will store the artifacts.
	ArtifactsFolder string `yaml:"artifactsFolder"`
	// ManifestURL is the URL to a custom manifest file for testing non-commercial partitions.
	// If specified, the hybrid nodes will download this manifest and pass it to nodeadm init
	// using the --manifest-override flag.
	ManifestURL string `yaml:"manifestURL"`
}

// ReadConfig reads the configuration from the specified file path and unmarshals it into the TestConfig struct.
func ReadConfig(configPath string) (*TestConfig, error) {
	config := &TestConfig{}
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading tests configuration file %s: %w", configPath, err)
	}

	if err = yaml.Unmarshal(file, config); err != nil {
		return nil, fmt.Errorf("unmarshaling test configuration: %w", err)
	}

	if config.ArtifactsFolder == "" {
		config.ArtifactsFolder = "/tmp"
	}

	return config, nil
}
