package iamrolesanywhere

import (
	"context"
	"fmt"

	"github.com/aws/eks-hybrid/internal/artifact"
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
		return fmt.Errorf("aws-iam-authenticator: %w", err)
	}
	defer authenticator.Close()

	if err := artifact.InstallFile(IAMAuthenticatorBinPath, authenticator, 0755); err != nil {
		return fmt.Errorf("aws-iam-authenticator: %w", err)
	}

	if !authenticator.VerifyChecksum() {
		return fmt.Errorf("aws-iam-authenticator: %w", artifact.NewChecksumError(authenticator))
	}

	return nil
}

func InstallSigningHelper(ctx context.Context, signingHelperSrc SigningHelperSource) error {
	signingHelper, err := signingHelperSrc.GetSigningHelper(ctx)
	if err != nil {
		return fmt.Errorf("aws_signing_helper: %w", err)
	}
	defer signingHelper.Close()

	if err := artifact.InstallFile(SigningHelperBinPath, signingHelper, 0755); err != nil {
		return fmt.Errorf("aws_signing_helper: %w", err)
	}

	if !signingHelper.VerifyChecksum() {
		return fmt.Errorf("aws_signing_helper: %w", artifact.NewChecksumError(signingHelper))
	}

	return nil
}
