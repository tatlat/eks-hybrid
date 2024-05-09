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

type checksumVerifier struct {
	expect []byte
	digest hash.Hash
}

type nopChecksumVerifier struct{}

type ChecksumError struct {
	Expect []byte
	Actual []byte
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

// NewChecksumError creates an error of type ChecksumError.
func NewChecksumError(src Source) error {
	return ChecksumError{
		Expect: src.ExpectedChecksum(),
		Actual: src.ActualChecksum(),
	}
}

// Actual satisfies Checksumer.
func (c checksumVerifier) ActualChecksum() []byte {
	return c.digest.Sum(nil)
}

// Expected satisfies Checksumer.
func (c checksumVerifier) ExpectedChecksum() []byte {
	return c.expect
}

// VerifyChecksum satisfies Checksumer. Calling verify before the associated Source has been
// completely read will cause an incomplete digest to be used and result in a false return.
func (c checksumVerifier) VerifyChecksum() bool {
	return bytes.Equal(c.ActualChecksum(), c.ExpectedChecksum())
}

// Error implements the error interface.
func (ce ChecksumError) Error() string {
	return fmt.Sprintf("checksum mismatch (expect != actual): %v != %v", ce.Expect, ce.Actual)
}

// Is implements the errors.Is interface.
func (ce ChecksumError) Is(err error) bool {
	_, ok := err.(ChecksumError)
	return ok
}

func (nopChecksumVerifier) VerifyChecksum() bool     { return true }
func (nopChecksumVerifier) ExpectedChecksum() []byte { return nil }
func (nopChecksumVerifier) ActualChecksum() []byte   { return nil }
