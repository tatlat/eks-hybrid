package iamrolesanywhere

import (
	"context"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

// SigingHelperBinPath is the path that the signing helper is installed to.
const SigningHelperBinPath = "/usr/local/bin/aws_signing_helper"

// IAMAuthenticatorBinPath is the path the IAM Authenticator is installed to.
const IAMAuthenticatorBinPath = "/usr/local/bin/aws-iam-authenticator"

// IAMAuthenticatorSource retrieves the aws-iam-authenticator binary.
type IAMAuthenticatorSource interface {
	GetIAMAuthenticator(context.Context) (artifact.Source, error)
}

// SigningHelperSource retrieves the aws_signing_helper binary.
type SigningHelperSource interface {
	GetSigningHelper(context.Context) (artifact.Source, error)
}

// Install installs the aws_signing_helper and aws-iam-authenticator on the system at
// SigningHelperBinPath and IAMAuthenticatorBinPath respectively.
func InstallIAMAuthenticator(ctx context.Context, iamAuthSrc IAMAuthenticatorSource) error {
	authenticator, err := iamAuthSrc.GetIAMAuthenticator(ctx)
	if err != nil {
		return err
	}
	defer authenticator.Close()

	return artifact.InstallFile(IAMAuthenticatorBinPath, authenticator, 0755)
}

func InstallSigningHelper(ctx context.Context, signingHelperSrc SigningHelperSource) error {
	signingHelper, err := signingHelperSrc.GetSigningHelper(ctx)
	if err != nil {
		return err
	}
	defer signingHelper.Close()

	return artifact.InstallFile(SigningHelperBinPath, signingHelper, 0755)
}
