package network

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/validation"
)

type NetworkInterfaceValidator struct {
	network     Network
	validateMTU bool
	cluster     *types.Cluster
}

func NewNetworkInterfaceValidator(opts ...func(*NetworkInterfaceValidator)) NetworkInterfaceValidator {
	v := &NetworkInterfaceValidator{
		network:     NewDefaultNetwork(),
		validateMTU: true, // Default to true
	}
	for _, opt := range opts {
		opt(v)
	}
	return *v
}

func WithNetwork(network Network) func(*NetworkInterfaceValidator) {
	return func(v *NetworkInterfaceValidator) {
		v.network = network
	}
}

func WithMTUValidation(validate bool) func(*NetworkInterfaceValidator) {
	return func(v *NetworkInterfaceValidator) {
		v.validateMTU = validate
	}
}

func WithCluster(cluster *types.Cluster) func(*NetworkInterfaceValidator) {
	return func(v *NetworkInterfaceValidator) {
		v.cluster = cluster
	}
}

func (v NetworkInterfaceValidator) Run(ctx context.Context, informer validation.Informer, node *api.NodeConfig) error {
	var err error
	name := "network-interface-validation"

	// Use provided cluster if available, otherwise read from EKS API
	if v.cluster == nil {
		informer.Starting(ctx, name, "Skipping network interface validation due to node IAM role missing EKS DescribeCluster permission")
		informer.Done(ctx, name, err)
		return nil
	}

	informer.Starting(ctx, name, "Validating hybrid node network interface")
	defer func() {
		informer.Done(ctx, name, err)
	}()

	if err = ValidateClusterRemoteNetworkConfig(v.cluster); err != nil {
		err = validation.WithRemediation(err,
			"Ensure the EKS cluster has remote network configuration set up properly. "+
				"The cluster must have remote node networks configured to validate hybrid node connectivity.")
		return err
	}

	// Get kubelet arguments from the node configuration
	kubeletArgs := node.Spec.Kubelet.Flags
	var iamNodeName string
	if node.IsIAMRolesAnywhere() {
		iamNodeName = node.Status.Hybrid.NodeName
	}

	// Get the node IP using the shared utility function
	nodeIP, err := GetNodeIP(kubeletArgs, iamNodeName, v.network)
	if err != nil {
		err = validation.WithRemediation(err,
			"Ensure the node has a valid network interface configuration. "+
				"Check that the node can resolve its hostname or has a valid --node-ip flag set. "+
				"See https://docs.aws.amazon.com/eks/latest/userguide/hybrid-nodes-troubleshooting.html")
		return err
	}

	// Validate that the node IP is in the remote node networks using shared utility function
	if err = ValidateIPInRemoteNodeNetwork(nodeIP, v.cluster.RemoteNetworkConfig.RemoteNodeNetworks); err != nil {
		err = validation.WithRemediation(err,
			"Ensure the node IP is within the configured remote network CIDR blocks. "+
				"Update the remote network configuration in the EKS cluster or adjust the node's network configuration. "+
				"See https://docs.aws.amazon.com/eks/latest/userguide/hybrid-nodes-troubleshooting.html")
		return err
	}

	// Validate MTU for the network interface associated with the node IP (if enabled)
	if v.validateMTU {
		if err = ValidateNetworkInterfaceMTUForIP(nodeIP); err != nil {
			err = validation.WithRemediation(err,
				"Ensure the network interface with the node IP has a valid MTU value. "+
					"MTU should be <= 1500 (standard Ethernet) or between 8000-9001 (jumbo frames). "+
					"Update the network interface configuration to use acceptable MTU values. "+
					"See https://docs.aws.amazon.com/vpc/latest/tgw/transit-gateway-quotas.html#mtu-quotas")
			return err
		}
	}

	return nil
}
