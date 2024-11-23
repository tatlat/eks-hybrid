package eks

import (
	eks_sdk "github.com/aws/aws-sdk-go-v2/service/eks/types"
	smithydocument "github.com/aws/smithy-go/document"
)

// An object representing an Amazon EKS cluster.
type Cluster struct {
	// The access configuration for the cluster.
	AccessConfig *eks_sdk.AccessConfigResponse

	// The Amazon Resource Name (ARN) of the cluster.
	Arn *string

	// The certificate-authority-data for your cluster.
	CertificateAuthority *eks_sdk.Certificate

	// A unique, case-sensitive identifier that you provide to ensure the idempotency
	// of the request.
	ClientRequestToken *string

	// The configuration used to connect to a cluster for registration.
	ConnectorConfig *eks_sdk.ConnectorConfigResponse

	// The encryption configuration for the cluster.
	EncryptionConfig []eks_sdk.EncryptionConfig

	// The endpoint for your Kubernetes API server.
	Endpoint *string

	// An object representing the health of your Amazon EKS cluster.
	Health *eks_sdk.ClusterHealth

	// The ID of your local Amazon EKS cluster on an Amazon Web Services Outpost. This
	// property isn't available for an Amazon EKS cluster on the Amazon Web Services
	// cloud.
	Id *string

	// The identity provider information for the cluster.
	Identity *eks_sdk.Identity

	// The Kubernetes network configuration for the cluster.
	KubernetesNetworkConfig *eks_sdk.KubernetesNetworkConfigResponse

	// The logging configuration for your cluster.
	Logging *eks_sdk.Logging

	// The name of your cluster.
	Name *string

	// An object representing the configuration of your local Amazon EKS cluster on an
	// Amazon Web Services Outpost. This object isn't available for clusters on the
	// Amazon Web Services cloud.
	OutpostConfig *eks_sdk.OutpostConfigResponse

	// The platform version of your Amazon EKS cluster. For more information about
	// clusters deployed on the Amazon Web Services Cloud, see [Platform versions]in the Amazon EKS User
	// Guide . For more information about local clusters deployed on an Outpost, see [Amazon EKS local cluster platform versions]
	// in the Amazon EKS User Guide .
	//
	// [Platform versions]: https://docs.aws.amazon.com/eks/latest/userguide/platform-versions.html
	// [Amazon EKS local cluster platform versions]: https://docs.aws.amazon.com/eks/latest/userguide/eks-outposts-platform-versions.html
	PlatformVersion *string

	// The VPC configuration used by the cluster control plane. Amazon EKS VPC
	// resources have specific requirements to work properly with Kubernetes. For more
	// information, see [Cluster VPC considerations]and [Cluster security group considerations] in the Amazon EKS User Guide.
	//
	// [Cluster security group considerations]: https://docs.aws.amazon.com/eks/latest/userguide/sec-group-reqs.html
	// [Cluster VPC considerations]: https://docs.aws.amazon.com/eks/latest/userguide/network_reqs.html
	ResourcesVpcConfig *eks_sdk.VpcConfigResponse

	// The Amazon Resource Name (ARN) of the IAM role that provides permissions for
	// the Kubernetes control plane to make calls to Amazon Web Services API operations
	// on your behalf.
	RoleArn *string

	// The current status of the cluster.
	Status eks_sdk.ClusterStatus

	// Metadata that assists with categorization and organization. Each tag consists
	// of a key and an optional value. You define both. Tags don't propagate to any
	// other cluster or Amazon Web Services resources.
	Tags map[string]string

	// The Kubernetes server version for the cluster.
	Version *string

	RemoteNetworkConfig *RemoteNetworkConfig `locationName:"remoteNetworkConfig" type:"structure"`

	smithydocument.NoSerde
}

type RemoteNetworkConfig struct {
	smithydocument.NoSerde

	RemoteNodeNetworks []*RemoteNodeNetwork `locationName:"remoteNodeNetworks" type:"list"`
	RemotePodNetworks  []*RemotePodNetwork  `locationName:"remotePodNetworks" type:"list"`
}

type RemoteNodeNetwork struct {
	smithydocument.NoSerde

	CIDRs []*string `locationName:"cidrs" type:"list"`
}

type RemotePodNetwork struct {
	smithydocument.NoSerde

	CIDRs []*string `locationName:"cidrs" type:"list"`
}
