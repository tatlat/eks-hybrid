package iamrolesanywhere_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/aws"
	"github.com/aws/eks-hybrid/internal/iamrolesanywhere"
	"github.com/aws/eks-hybrid/internal/test"
	"github.com/aws/eks-hybrid/internal/tracker"
)

func TestInstall(t *testing.T) {
	iamrolesanywhereData := []byte("test aws_signing_helper binary")

	test.RunInstallTest(t, test.TestData{
		ArtifactName: "aws_signing_helper",
		BinaryName:   "aws_signing_helper",
		Data:         iamrolesanywhereData,
		Install: func(ctx context.Context, tempDir string, source aws.Source, tr *tracker.Tracker) error {
			return iamrolesanywhere.Install(ctx, iamrolesanywhere.InstallOptions{
				InstallRoot: tempDir,
				Tracker:     tr,
				Source:      source,
				Logger:      zap.NewNop(),
			})
		},
		Verify: func(g *GomegaWithT, tempDir string, tr *tracker.Tracker) {
			g.Expect(tr.Artifacts.IamRolesAnywhere).To(BeTrue())
		},
		VerifyFilePaths: []string{iamrolesanywhere.SigningHelperBinPath},
	})
}
