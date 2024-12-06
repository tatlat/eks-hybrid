package e2e

import (
	"regexp"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

func isErrCode(err error, code string) bool {
	if err == nil {
		return false
	}
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == code
	}

	return false
}

// SanitizeForAWSName removes everything except alphanumeric characters and hyphens from a string.
func SanitizeForAWSName(input string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
	return re.ReplaceAllString(input, "")
}
