package artifact_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"strings"
	"testing"

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
