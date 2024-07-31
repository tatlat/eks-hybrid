package iamrolesanywhere

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path"
	"text/template"
)

const (
	// DefaultAWSConfigPath is the path where the AWS config is written.
	DefaultAWSConfigPath = "/etc/aws/hybrid/config"

	// ProfileName is the profile used when writing the AWS config.
	ProfileName = "hybrid"
)

//go:embed aws_config.tpl
var unformattedRawAWSConfigTpl string

var rawAWSConfigTpl = fmt.Sprintf(unformattedRawAWSConfigTpl, ProfileName)

var awsConfigTpl = template.Must(template.New("").Parse(rawAWSConfigTpl))

// AWSConfig defines the data for configuring IAM Roles Anywhere AWS Configuration files.
type AWSConfig struct {
	// TrustAnchorARN is the ARN of the trust anchor for IAM Roles Anywhere.
	TrustAnchorARN string

	// ProfileARN is the ARN of the profile for IAM Roles Anywhere.
	ProfileARN string

	// RoleARN is the role to assume after auth.
	RoleARN string

	// Region is the region to target when authenticating.
	Region string

	// ConfigPath is a path to a configuration file to be verified. Defaults to /etc/aws/hybrid/profile.
	ConfigPath string

	// SigningHelperBinPath is a pth to the aws iam roles anywhere signer helper. Defaults to /usr/local/bin/aws_signing_helper
	SigningHelperBinPath string
}

// EnsureAWSConfig ensures an AWS configuration file with contents appropriate for cfg exists
// at cfg.ConfigPath.
func EnsureAWSConfig(cfg AWSConfig) error {
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = DefaultAWSConfigPath
	}

	if err := validateAWSConfig(cfg); err != nil {
		return err
	}

	exists, err := configFileExists(cfg)
	if err != nil {
		return err
	}

	if exists {
		return verifyConfigFile(cfg)
	}

	return writeConfigFile(cfg)
}

func validateAWSConfig(cfg AWSConfig) error {
	var errs []error

	if cfg.TrustAnchorARN == "" {
		errs = append(errs, errors.New("TrustAnchorARN cannot be empty"))
	}

	if cfg.ProfileARN == "" {
		errs = append(errs, errors.New("ProfileARN cannot be empty"))
	}

	if cfg.RoleARN == "" {
		errs = append(errs, errors.New("RoleARN cannot be empty"))
	}

	if cfg.Region == "" {
		errs = append(errs, errors.New("Region cannot be empty"))
	}

	if cfg.SigningHelperBinPath == "" {
		errs = append(errs, errors.New("Singing helper path cannot be emtpyy"))
	}

	return errors.Join(errs...)
}

func configFileExists(cfg AWSConfig) (bool, error) {
	_, err := os.Stat(cfg.ConfigPath)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func verifyConfigFile(cfg AWSConfig) error {
	var buf bytes.Buffer
	if err := awsConfigTpl.Execute(&buf, cfg); err != nil {
		return err
	}

	tplChecksum := sha256.Sum256(buf.Bytes())

	configContents, err := getConfigFileContents(cfg)
	if err != nil {
		return err
	}

	profileChecksum := sha256.Sum256(configContents)

	if tplChecksum != profileChecksum {
		return fmt.Errorf("hybrid profile already exists at %v but its contents do not align with the expected configuration", cfg.ConfigPath)
	}

	return nil
}

func getConfigFileContents(cfg AWSConfig) ([]byte, error) {
	fh, err := os.Open(cfg.ConfigPath)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	// Protect against a malicious file by doing a chunk read. With ARNs we don't expect more than
	// 500 bytes so 2048 should be plenty.
	rawBuf := make([]byte, 2048)
	readCount, err := fh.Read(rawBuf)
	if err != nil {
		return nil, err
	}
	if readCount == 2048 {
		return nil, fmt.Errorf("unexpected amount of data in file: %v", cfg.ConfigPath)
	}

	return rawBuf[:readCount], nil
}

func writeConfigFile(cfg AWSConfig) error {
	var buf bytes.Buffer
	if err := awsConfigTpl.Execute(&buf, cfg); err != nil {
		return err
	}

	if err := os.MkdirAll(path.Dir(cfg.ConfigPath), os.ModeDir); err != nil {
		return err
	}

	if err := os.WriteFile(cfg.ConfigPath, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
