//go:build e2e
// +build e2e

package e2e

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

// newE2EAWSSession constructs AWS session for E2E tests.
func newE2EAWSSession(region string) (*session.Session, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("creating AWS session: %w", err)
	}
	return sess, nil
}
