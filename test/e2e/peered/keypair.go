package peered

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/eks-hybrid/test/e2e/constants"
)

const maxKeyPairWaitTime = 2 * time.Minute

// KeyPair returns the keypair for the given cluster.
func KeyPair(ctx context.Context, client *ec2.Client, clusterName string) (*types.KeyPairInfo, error) {
	waiter := ec2.NewKeyPairExistsWaiter(client)
	keyPair, err := waiter.WaitForOutput(ctx, &ec2.DescribeKeyPairsInput{
		IncludePublicKey: aws.Bool(true),
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + constants.TestClusterTagKey),
				Values: []string{clusterName},
			},
		},
	}, maxKeyPairWaitTime)
	if err != nil {
		return nil, err
	}
	if len(keyPair.KeyPairs) == 0 {
		return nil, fmt.Errorf("no key pair found for cluster %s", clusterName)
	}
	return &keyPair.KeyPairs[0], nil
}
