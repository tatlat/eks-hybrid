package e2e

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
)

const assumeRolePolicyDocument = `{
	"Version": "2012-10-17",
	"Statement": [
	  {
		"Effect": "Allow",
		"Principal": {
		  "Service": "eks.amazonaws.com"
		},
		"Action": "sts:AssumeRole"
	  }
	]
  }`

const eksClusterPolicyArn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"

func (config *ClusterConfig) createEKSClusterRole() (string, error) {
	svc := iam.New(config.Session)
	roleName := fmt.Sprintf("%s-eks-role", config.ClusterName)

	// Create IAM role
	role, err := svc.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create role: %v", err)
	}

	// Attach AmazonEKSClusterPolicy
	_, err = svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(eksClusterPolicyArn),
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach policy: %v", err)
	}

	fmt.Printf("Successfully created IAM role: %s\n", *role.Role.Arn)
	return *role.Role.Arn, nil
}
