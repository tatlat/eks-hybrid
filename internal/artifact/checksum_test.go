package artifact_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/artifact"
)

func TestVerifyChecksum(t *testing.T) {
	data := "hello world"
	digest := sha256.New()

	// Calculate the expected digest for the data.
	_, err := io.Copy(digest, bytes.NewBufferString(data))
	if err != nil {
		t.Fatal(err)
	}

	expect := digest.Sum(nil)

	t.Run("NoChecksum", func(t *testing.T) {
		src := bytes.NewBufferString(data)
		if err := artifact.VerifyChecksum(src); err != nil {
			t.Fatalf("Expected nil; received %v", err)
		}
	})

	t.Run("GoodChecksum", func(t *testing.T) {
		buf := bytes.NewBufferString(data)
		src := artifact.WithChecksum(buf, sha256.New(), expect)
		if _, err := io.Copy(io.Discard, src); err != nil {
			t.Fatal(err)
		}
		if err := artifact.VerifyChecksum(src); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BadChecksum", func(t *testing.T) {
		buf := bytes.NewBufferString(data)
		src := artifact.WithChecksum(buf, sha256.New(), []byte("expect mismatch"))
		if _, err := io.Copy(io.Discard, src); err != nil {
			t.Fatal(err)
		}
		if err := artifact.VerifyChecksum(src); err == nil {
			t.Fatalf("Expected checksum mismatch but received match")
		}
	})
}

func TestParseGNUChecksum(t *testing.T) {
	buf := bytes.NewBufferString("aaaa file")
	checksum, err := artifact.ParseGNUChecksum(buf)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(checksum, []byte("aaaa")) {
		t.Fatalf("Received unexpected checksum: %v", checksum)
	}
}
