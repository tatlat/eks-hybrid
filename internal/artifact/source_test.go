package artifact_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"io"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/artifact"
)

func TestWithChecksum(t *testing.T) {
	data := "hello world"

	expect := []byte("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9  -")
	mismatchExpect := []byte("2fe4d4a5963f28b77737c091c436096beee0b74fabb9fcdcd2a4d8859d2099a3  -")

	t.Run("GoodChecksum", func(t *testing.T) {
		src, err := artifact.WithChecksum(io.NopCloser(bytes.NewBufferString(data)), sha256.New(), expect)
		if err != nil {
			t.Fatal(err)
		}

		_, err = io.Copy(io.Discard, src)
		if err != nil {
			t.Fatal(err)
		}

		if !src.VerifyChecksum() {
			t.Fatalf("Expected true; expect = %x; actual = %x", src.ExpectedChecksum(), src.ActualChecksum())
		}
	})

	t.Run("BadChecksum", func(t *testing.T) {
		src, err := artifact.WithChecksum(io.NopCloser(bytes.NewBufferString(data)), sha256.New(), mismatchExpect)
		if err != nil {
			t.Fatal(err)
		}

		_, err = io.Copy(io.Discard, src)
		if err != nil {
			t.Fatal(err)
		}

		if src.VerifyChecksum() {
			t.Fatalf("Expected true; expect = %x; actual = %x", src.ExpectedChecksum(), src.ActualChecksum())
		}
	})
}

func TestWithNopChecksum(t *testing.T) {
	t.Run("DataRead", func(t *testing.T) {
		src := artifact.WithNopChecksum(io.NopCloser(strings.NewReader("hello world")))

		_, err := io.Copy(io.Discard, src)
		if err != nil {
			t.Fatal(err)
		}

		if !src.VerifyChecksum() {
			t.Fatalf("Expected true; expect = %x; actual = %x", src.ExpectedChecksum(), src.ActualChecksum())
		}
	})

	t.Run("NoDataRead", func(t *testing.T) {
		src := artifact.WithNopChecksum(io.NopCloser(strings.NewReader("hello world")))

		_, err := io.Copy(io.Discard, src)
		if err != nil {
			t.Fatal(err)
		}

		if !src.VerifyChecksum() {
			t.Fatalf("Expected true; expect = %x; actual = %x", src.ExpectedChecksum(), src.ActualChecksum())
		}
	})
}

func TestGzippedWithChecksum(t *testing.T) {
	g := NewWithT(t)
	data := "hello world"
	expect := []byte("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9  -")
	mismatchExpect := []byte("2fe4d4a5963f28b77737c091c436096beee0b74fabb9fcdcd2a4d8859d2099a3  -")

	// Create gzipped data
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write([]byte(data))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gw.Close()).To(Succeed())
	gzippedData := buf.Bytes()

	t.Run("GoodChecksum", func(t *testing.T) {
		g := NewWithT(t)
		src, err := artifact.GzippedWithChecksum(io.NopCloser(bytes.NewBuffer(gzippedData)), sha256.New(), expect)
		g.Expect(err).NotTo(HaveOccurred())

		_, err = io.Copy(io.Discard, src)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(src.VerifyChecksum()).To(BeTrue(), "checksum verification should succeed")
	})

	t.Run("BadChecksum", func(t *testing.T) {
		g := NewWithT(t)
		src, err := artifact.GzippedWithChecksum(io.NopCloser(bytes.NewBuffer(gzippedData)), sha256.New(), mismatchExpect)
		g.Expect(err).NotTo(HaveOccurred())

		_, err = io.Copy(io.Discard, src)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(src.VerifyChecksum()).To(BeFalse(), "checksum verification should fail")
	})

	t.Run("InvalidGzip", func(t *testing.T) {
		g := NewWithT(t)
		_, err := artifact.GzippedWithChecksum(io.NopCloser(bytes.NewBufferString("not gzipped")), sha256.New(), expect)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("getting gzip reader"))
	})
}
