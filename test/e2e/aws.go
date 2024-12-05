package e2e

import (
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
