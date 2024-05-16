package artifact_test

import (
	"bytes"
	"testing"

	"github.com/aws/eks-hybrid/internal/artifact"
)

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
