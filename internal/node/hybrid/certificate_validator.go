package hybrid

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
)

type ValidationErrorType int

const (
	ErrorNoCert ValidationErrorType = iota
	ErrorCertFile
	ErrorReadFile
	ErrorInvalidFormat
	ErrorClockSkewDetected
	ErrorExpired
	ErrorParseCA
	ErrorInvalidCA
)

// ValidationError represents a kubelet certificate validation error with remediation information
type ValidationError struct {
	message             string
	cause               error
	validationErrorType ValidationErrorType
}

func (e *ValidationError) ErrorType() ValidationErrorType {
	return e.validationErrorType
}

func (e *ValidationError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

func (e *ValidationError) Unwrap() error {
	return e.cause
}

func IsDateValidationError(err error) bool {
	var v *ValidationError
	return errors.As(err, &v) && (v.validationErrorType == ErrorClockSkewDetected || v.validationErrorType == ErrorExpired)
}

func IsNoCertError(err error) bool {
	var v *ValidationError
	return errors.As(err, &v) && v.validationErrorType == ErrorNoCert
}

// ValidateCertificate checks if there is an existing certificate and validates it against the provided CA
func ValidateCertificate(certPath string, ca []byte) error {
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		// Return an error for no cert, but one that can be identified
		return &ValidationError{
			message:             "no certificate found",
			validationErrorType: ErrorNoCert,
		}
	} else if err != nil {
		return &ValidationError{
			message:             "checking certificate",
			cause:               err,
			validationErrorType: ErrorCertFile,
		}
	}

	certData, err := os.ReadFile(certPath)
	if err != nil {
		return &ValidationError{
			message:             "reading certificate",
			cause:               err,
			validationErrorType: ErrorReadFile,
		}
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return &ValidationError{
			message:             "parsing certificate",
			validationErrorType: ErrorInvalidFormat,
		}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return &ValidationError{
			message:             "parsing certificate",
			cause:               err,
			validationErrorType: ErrorInvalidFormat,
		}
	}

	now := time.Now()
	if now.Before(cert.NotBefore) {
		return &ValidationError{
			message:             "server certificate is not yet valid",
			validationErrorType: ErrorClockSkewDetected,
		}
	}

	if now.After(cert.NotAfter) {
		return &ValidationError{
			message:             "server certificate has expired",
			validationErrorType: ErrorExpired,
		}
	}

	if len(ca) > 0 {
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(ca) {
			return &ValidationError{
				message:             "parsing cluster CA certificate",
				validationErrorType: ErrorParseCA,
			}
		}

		opts := x509.VerifyOptions{
			Roots:       caPool,
			CurrentTime: now,
		}

		if _, err := cert.Verify(opts); err != nil {
			return &ValidationError{
				message:             "certificate is not valid for the current cluster",
				cause:               err,
				validationErrorType: ErrorInvalidCA,
			}
		}
	}

	return nil
}
