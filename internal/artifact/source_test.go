package artifact_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"strings"
	"testing"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

func TestWithChecksum(t *testing.T) {
	data := "hello world"
	digest := sha256.New()

	// Calculate the expected digest for the data.
	_, err := io.Copy(digest, bytes.NewBufferString(data))
	if err != nil {
		t.Fatal(err)
	}
	expect := digest.Sum(nil)

	t.Run("GoodChecksum", func(t *testing.T) {
		src := artifact.WithChecksum(io.NopCloser(bytes.NewBufferString(data)), sha256.New(), expect)

		_, err := io.Copy(io.Discard, src)
		if err != nil {
			t.Fatal(err)
		}

		if !src.VerifyChecksum() {
			t.Fatalf("Expected true; expect = %x; actual = %x", src.ExpectedChecksum(), src.ActualChecksum())
		}
	})

	t.Run("BadChecksum", func(t *testing.T) {
		src := artifact.WithChecksum(io.NopCloser(bytes.NewBufferString(data)), sha256.New(), []byte("mismatch"))

		_, err := io.Copy(io.Discard, src)
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
