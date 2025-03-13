package artifact

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

func TestUpgradeAvailable(t *testing.T) {
	dummyFilePath := "testdata/dummyfile"
	dummyFh, err := os.Open(dummyFilePath)
	if err != nil {
		t.Fatal(err)
	}
	fileChecksum := []byte("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9  internal/artifact/testdata/dummyfile")
	wrongChecksum := []byte("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7acabcdcde9 randomfile/path")

	testcases := []struct {
		name           string
		filePath       string
		sourceChecksum []byte
		checksumMatch  bool
		wantErr        error
	}{
		{
			name:           "Upgrade available",
			filePath:       dummyFilePath,
			sourceChecksum: wrongChecksum,
			checksumMatch:  false,
			wantErr:        nil,
		},
		{
			name:           "Upgrade not available",
			filePath:       dummyFilePath,
			sourceChecksum: fileChecksum,
			checksumMatch:  true,
			wantErr:        nil,
		},
		{
			name:           "File not installed",
			filePath:       "wrong/path",
			sourceChecksum: wrongChecksum,
			checksumMatch:  false,
			wantErr:        fmt.Errorf("checking for checksum match: open wrong/path: no such file or directory"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			src, err := WithChecksum(dummyFh, sha256.New(), tc.sourceChecksum)
			if err != nil {
				g.Expect(err).To(BeNil())
			}
			available, err := checksumMatch(tc.filePath, src)
			if err != nil {
				g.Expect(err.Error()).To(Equal(tc.wantErr.Error()))
			}
			g.Expect(available).To(Equal(tc.checksumMatch))
		})
	}
}

func TestUpgrade(t *testing.T) {
	g := NewGomegaWithT(t)
	installedFileData := "hello world"
	fileChecksum := []byte("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9 -")

	upgradedFileData := "random text data"
	upgradedChecksum := []byte("2ea385d95836d380760082c32f7e2a76d0a00a6e4864cb2a5b534e3a6750c0ab -")

	testcases := []struct {
		name           string
		installedData  string
		upgradedData   string
		sourceChecksum []byte
	}{
		{
			name:           "Same file, nothing to upgrade",
			installedData:  installedFileData,
			upgradedData:   installedFileData,
			sourceChecksum: fileChecksum,
		},
		{
			name:           "Upgrade available, should upgrade",
			installedData:  installedFileData,
			upgradedData:   upgradedFileData,
			sourceChecksum: upgradedChecksum,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			artifact, err := os.CreateTemp(tmpDir, "installedArtifact")
			g.Expect(err).To(BeNil())
			_, err = artifact.WriteString(tt.installedData)
			g.Expect(err).To(BeNil())

			source, err := WithChecksum(io.NopCloser(bytes.NewBufferString(tt.upgradedData)), sha256.New(), tt.sourceChecksum)
			g.Expect(err).To(BeNil())

			err = Upgrade("dummyArtifact", artifact.Name(), source, 0o755, zap.NewNop())
			g.Expect(err).To(BeNil())

			// Check upgraded written data
			fileData, err := os.ReadFile(artifact.Name())
			if err != nil {
				t.Fatal(err)
			}
			g.Expect(fileData).To(Equal([]byte(tt.upgradedData)))
		})
	}
}
