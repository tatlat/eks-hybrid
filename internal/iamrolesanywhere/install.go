package iamrolesanywhere

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/eks-hybrid/internal/artifact"
	"github.com/aws/eks-hybrid/internal/tracker"
)

// SigingHelperBinPath is the path that the signing helper is installed to.
const SigningHelperBinPath = "/usr/local/bin/aws_signing_helper"

// SigningHelperSource retrieves the aws_signing_helper binary.
type SigningHelperSource interface {
	GetSigningHelper(context.Context) (artifact.Source, error)
}

func Install(ctx context.Context, tracker *tracker.Tracker, signingHelperSrc SigningHelperSource) error {
	signingHelper, err := signingHelperSrc.GetSigningHelper(ctx)
	if err != nil {
		return fmt.Errorf("aws_signing_helper: %w", err)
	}
	defer signingHelper.Close()

	if err := artifact.InstallFile(SigningHelperBinPath, signingHelper, 0755); err != nil {
		return fmt.Errorf("aws_signing_helper: %w", err)
	}
	if err = tracker.Add(artifact.IamRolesAnywhere); err != nil {
		return err
	}

	if !signingHelper.VerifyChecksum() {
		return fmt.Errorf("aws_signing_helper: %w", artifact.NewChecksumError(signingHelper))
	}

	return nil
}

func Uninstall() error {
	return os.RemoveAll(SigningHelperBinPath)
}
