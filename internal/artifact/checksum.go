package artifact

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"strings"
)

var _ Checksummer = checksum{}

// Checksummer is a Source that has an associated checksum.
type Checksummer interface {
	io.Reader

	// ActualChecksum returns the checksum calculated from the bytes that have been read from
	// Source.
	ActualChecksum() []byte

	// Expected checksum returns the expected checksum that should be returned from Checksum once
	// all bytes are read from the underlying Source.
	ExpectChecksum() []byte

	// VerifyChecksum verifies if ActualChecksum equals ExpectChecksum.
	VerifyChecksum() bool
}

type ChecksumError struct {
	Expect []byte
	Actual []byte
}

type checksum struct {
	io.Reader
	Expect []byte
	Digest hash.Hash
}

// ParseGNUChecksum parses r as GNU checksum format and returns the checksum value. GNU checksums
// are a space separated digest and filename:
//
//	<digest> <filename>
func ParseGNUChecksum(r io.Reader) ([]byte, error) {
	var buf strings.Builder
	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, err
	}

	ch, _, found := strings.Cut(buf.String(), " ")
	if !found {
		return nil, errors.New("invalid gnu checksum")
	}

	checksum, err := hex.DecodeString(ch)
	if err != nil {
		return nil, err
	}

	return checksum, nil
}

// WithChecksum creates a Source from src that can be type asserted to Checksumer. The checksum
// is calculated by arranging for digest to receive the bytes read from src. The source can be
// used with VerifyChecksum. expect is the expected hexidecimal value.
func WithChecksum(src io.Reader, digest hash.Hash, expect []byte) Checksummer {
	return checksum{
		Reader: io.TeeReader(src, digest),
		Expect: expect,
		Digest: digest,
	}
}

// VerifyChecksum will verify src satisfies the Checksumer interface. If it does, it will verify
// the expected checksum equals the received checksum. Calling VerifyChecksum on a Source before
// all bytes have been read is undefined. If src does not satisfy Checksumer it returns true.
func VerifyChecksum(src io.Reader) error {
	cs, ok := src.(Checksummer)
	if !ok {
		return nil
	}

	if !cs.VerifyChecksum() {
		return newChecksumError(cs)
	}

	return nil
}

func newChecksumError(c Checksummer) error {
	return ChecksumError{
		Expect: c.ExpectChecksum(),
		Actual: c.ActualChecksum(),
	}
}

// Checksum satisfies Checksumer.
func (c checksum) ActualChecksum() []byte {
	return c.Digest.Sum(nil)
}

// ExpectedChecksum satisfies Checksumer.
func (c checksum) ExpectChecksum() []byte {
	return c.Expect
}

// VerifyChecksum satisfies Checksumer.
func (c checksum) VerifyChecksum() bool {
	return bytes.Equal(c.ActualChecksum(), c.ExpectChecksum())
}

func (e ChecksumError) Error() string {
	return fmt.Sprintf("checksum mismatch: expect %x; actual %x", e.Expect, e.Actual)
}

func (e ChecksumError) Is(cmp error) bool {
	_, ok := cmp.(ChecksumError)
	return ok
}
