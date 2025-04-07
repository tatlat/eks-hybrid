package cni_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/aws/eks-hybrid/internal/aws"
	"github.com/aws/eks-hybrid/internal/cni"
	"github.com/aws/eks-hybrid/internal/test"
	"github.com/aws/eks-hybrid/internal/tracker"
)

func tarGzFile(g *GomegaWithT) []byte {
	// create a tar.gz file with no content
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	err := tw.WriteHeader(&tar.Header{
		Name: "cni-plugins.tgz",
		Mode: 0o644,
		Size: 0,
	})

	g.Expect(err).NotTo(HaveOccurred())

	hdr := &tar.Header{
		Name: "fake-plugin",
		Size: 0,
	}
	g.Expect(tw.WriteHeader(hdr)).NotTo(HaveOccurred())
	g.Expect(tw.Write([]byte(""))).Error().NotTo(HaveOccurred())

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestInstall(t *testing.T) {
	g := NewGomegaWithT(t)
	test.RunInstallTest(t, test.TestData{
		ArtifactName: "cni-plugins",
		BinaryName:   "cni-plugins.tgz",
		Data:         tarGzFile(g),
		Install: func(ctx context.Context, tempDir string, source aws.Source, tr *tracker.Tracker) error {
			return cni.Install(ctx, cni.InstallOptions{
				Tracker:     tr,
				Source:      source,
				Logger:      zap.NewNop(),
				InstallRoot: tempDir,
			})
		},
		Verify: func(g *GomegaWithT, tempDir string, tr *tracker.Tracker) {
			g.Expect(tr.Artifacts.CniPlugins).To(BeTrue())
		},
		VerifyFilePaths: []string{filepath.Join(cni.BinPath, "fake-plugin")},
	})
}
