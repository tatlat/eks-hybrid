package creds

import (
	"fmt"

	"github.com/aws/eks-hybrid/internal/api"
)

type CredentialProvider string

const (
	SsmCredentialProvider              CredentialProvider = "ssm"
	IamRolesAnywhereCredentialProvider CredentialProvider = "iam-ra"
)

func GetCredentialProvider(credProcess string) (CredentialProvider, error) {
	switch credProcess {
	case string(SsmCredentialProvider):
		return SsmCredentialProvider, nil
	case string(IamRolesAnywhereCredentialProvider):
		return IamRolesAnywhereCredentialProvider, nil
	default:
		return "", fmt.Errorf("invalid credential process provided. Valid options are ssm and iam")
	}
}

func GetCredentialProviderFromNodeConfig(nodeCfg *api.NodeConfig) (CredentialProvider, error) {
	if nodeCfg.IsSSM() {
		return SsmCredentialProvider, nil
	} else if nodeCfg.IsIAMRolesAnywhere() {
		return IamRolesAnywhereCredentialProvider, nil
	}
	return "", fmt.Errorf("no credential process provided in nodeConfig")
}
