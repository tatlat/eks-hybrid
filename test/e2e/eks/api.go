package eks

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eks"
)

type CreateClusterInput struct {
	_ struct{} `type:"structure"`

	// The access configuration for the cluster.
	AccessConfig *eks.CreateAccessConfigRequest `locationName:"accessConfig" type:"structure"`

	// If you set this value to False when creating a cluster, the default networking
	// add-ons will not be installed.
	//
	// The default networking addons include vpc-cni, coredns, and kube-proxy.
	//
	// Use this option when you plan to install third-party alternative add-ons
	// or self-manage the default networking add-ons.
	BootstrapSelfManagedAddons *bool `locationName:"bootstrapSelfManagedAddons" type:"boolean"`

	// A unique, case-sensitive identifier that you provide to ensure the idempotency
	// of the request.
	ClientRequestToken *string `locationName:"clientRequestToken" type:"string" idempotencyToken:"true"`

	// The encryption configuration for the cluster.
	EncryptionConfig []*eks.EncryptionConfig `locationName:"encryptionConfig" type:"list"`

	// The Kubernetes network configuration for the cluster.
	KubernetesNetworkConfig *eks.KubernetesNetworkConfigRequest `locationName:"kubernetesNetworkConfig" type:"structure"`

	// Enable or disable exporting the Kubernetes control plane logs for your cluster
	// to CloudWatch Logs. By default, cluster control plane logs aren't exported
	// to CloudWatch Logs. For more information, see Amazon EKS Cluster control
	// plane logs (https://docs.aws.amazon.com/eks/latest/userguide/control-plane-logs.html)
	// in the Amazon EKS User Guide .
	//
	// CloudWatch Logs ingestion, archive storage, and data scanning rates apply
	// to exported control plane logs. For more information, see CloudWatch Pricing
	// (http://aws.amazon.com/cloudwatch/pricing/).
	Logging *eks.Logging `locationName:"logging" type:"structure"`

	// The unique name to give to your cluster.
	//
	// Name is a required field
	Name *string `locationName:"name" min:"1" type:"string" required:"true"`

	// An object representing the configuration of your local Amazon EKS cluster
	// on an Amazon Web Services Outpost. Before creating a local cluster on an
	// Outpost, review Local clusters for Amazon EKS on Amazon Web Services Outposts
	// (https://docs.aws.amazon.com/eks/latest/userguide/eks-outposts-local-cluster-overview.html)
	// in the Amazon EKS User Guide. This object isn't available for creating Amazon
	// EKS clusters on the Amazon Web Services cloud.
	OutpostConfig *eks.OutpostConfigRequest `locationName:"outpostConfig" type:"structure"`

	// The VPC configuration that's used by the cluster control plane. Amazon EKS
	// VPC resources have specific requirements to work properly with Kubernetes.
	// For more information, see Cluster VPC Considerations (https://docs.aws.amazon.com/eks/latest/userguide/network_reqs.html)
	// and Cluster Security Group Considerations (https://docs.aws.amazon.com/eks/latest/userguide/sec-group-reqs.html)
	// in the Amazon EKS User Guide. You must specify at least two subnets. You
	// can specify up to five security groups. However, we recommend that you use
	// a dedicated security group for your cluster control plane.
	//
	// ResourcesVpcConfig is a required field
	ResourcesVpcConfig *eks.VpcConfigRequest `locationName:"resourcesVpcConfig" type:"structure" required:"true"`

	// The Amazon Resource Name (ARN) of the IAM role that provides permissions
	// for the Kubernetes control plane to make calls to Amazon Web Services API
	// operations on your behalf. For more information, see Amazon EKS Service IAM
	// Role (https://docs.aws.amazon.com/eks/latest/userguide/service_IAM_role.html)
	// in the Amazon EKS User Guide .
	//
	// RoleArn is a required field
	RoleArn *string `locationName:"roleArn" type:"string" required:"true"`

	// Metadata that assists with categorization and organization. Each tag consists
	// of a key and an optional value. You define both. Tags don't propagate to
	// any other cluster or Amazon Web Services resources.
	Tags map[string]*string `locationName:"tags" min:"1" type:"map"`

	// New clusters, by default, have extended support enabled. You can disable
	// extended support when creating a cluster by setting this value to STANDARD.
	UpgradePolicy *eks.UpgradePolicyRequest `locationName:"upgradePolicy" type:"structure"`

	// The desired Kubernetes version for your cluster. If you don't specify a value
	// here, the default version available in Amazon EKS is used.
	//
	// The default version might not be the latest version available.
	Version             *string              `locationName:"version" type:"string"`
	RemoteNetworkConfig *RemoteNetworkConfig `locationName:"remoteNetworkConfig" type:"structure"`
}

type RemoteNetworkConfig struct {
	_ struct{} `type:"structure"`

	RemoteNodeNetworks []*RemoteNodeNetwork `locationName:"remoteNodeNetworks" type:"list"`
	RemotePodNetworks  []*RemotePodNetwork  `locationName:"remotePodNetworks" type:"list"`
}

type RemoteNodeNetwork struct {
	_ struct{} `type:"structure"`

	CIDRs []*string `locationName:"cidrs" type:"list"`
}

type RemotePodNetwork struct {
	_ struct{} `type:"structure"`

	CIDRs []*string `locationName:"cidrs" type:"list"`
}

type CreateClusterOutput struct {
	_ struct{} `type:"structure"`

	// The full description of your specified cluster.
	Cluster *Cluster `locationName:"cluster" type:"structure"`
}

// An object representing an Amazon EKS cluster.
type Cluster struct {
	_ struct{} `type:"structure"`

	// The access configuration for the cluster.
	AccessConfig *eks.AccessConfigResponse `locationName:"accessConfig" type:"structure"`

	// The Amazon Resource Name (ARN) of the cluster.
	Arn *string `locationName:"arn" type:"string"`

	// The certificate-authority-data for your cluster.
	CertificateAuthority *eks.Certificate `locationName:"certificateAuthority" type:"structure"`

	// A unique, case-sensitive identifier that you provide to ensure the idempotency
	// of the request.
	ClientRequestToken *string `locationName:"clientRequestToken" type:"string"`

	// The configuration used to connect to a cluster for registration.
	ConnectorConfig *eks.ConnectorConfigResponse `locationName:"connectorConfig" type:"structure"`

	// The Unix epoch timestamp at object creation.
	CreatedAt *time.Time `locationName:"createdAt" type:"timestamp"`

	// The encryption configuration for the cluster.
	EncryptionConfig []*eks.EncryptionConfig `locationName:"encryptionConfig" type:"list"`

	// The endpoint for your Kubernetes API server.
	Endpoint *string `locationName:"endpoint" type:"string"`

	// An object representing the health of your Amazon EKS cluster.
	Health *eks.ClusterHealth `locationName:"health" type:"structure"`

	// The ID of your local Amazon EKS cluster on an Amazon Web Services Outpost.
	// This property isn't available for an Amazon EKS cluster on the Amazon Web
	// Services cloud.
	Id *string `locationName:"id" type:"string"`

	// The identity provider information for the cluster.
	Identity *eks.Identity `locationName:"identity" type:"structure"`

	// The Kubernetes network configuration for the cluster.
	KubernetesNetworkConfig *eks.KubernetesNetworkConfigResponse `locationName:"kubernetesNetworkConfig" type:"structure"`

	// The logging configuration for your cluster.
	Logging *eks.Logging `locationName:"logging" type:"structure"`

	// The name of your cluster.
	Name *string `locationName:"name" type:"string"`

	// An object representing the configuration of your local Amazon EKS cluster
	// on an Amazon Web Services Outpost. This object isn't available for clusters
	// on the Amazon Web Services cloud.
	OutpostConfig *eks.OutpostConfigResponse `locationName:"outpostConfig" type:"structure"`

	// The platform version of your Amazon EKS cluster. For more information about
	// clusters deployed on the Amazon Web Services Cloud, see Platform versions
	// (https://docs.aws.amazon.com/eks/latest/userguide/platform-versions.html)
	// in the Amazon EKS User Guide . For more information about local clusters
	// deployed on an Outpost, see Amazon EKS local cluster platform versions (https://docs.aws.amazon.com/eks/latest/userguide/eks-outposts-platform-versions.html)
	// in the Amazon EKS User Guide .
	PlatformVersion *string `locationName:"platformVersion" type:"string"`

	// The VPC configuration used by the cluster control plane. Amazon EKS VPC resources
	// have specific requirements to work properly with Kubernetes. For more information,
	// see Cluster VPC considerations (https://docs.aws.amazon.com/eks/latest/userguide/network_reqs.html)
	// and Cluster security group considerations (https://docs.aws.amazon.com/eks/latest/userguide/sec-group-reqs.html)
	// in the Amazon EKS User Guide.
	ResourcesVpcConfig *eks.VpcConfigResponse `locationName:"resourcesVpcConfig" type:"structure"`

	// The Amazon Resource Name (ARN) of the IAM role that provides permissions
	// for the Kubernetes control plane to make calls to Amazon Web Services API
	// operations on your behalf.
	RoleArn *string `locationName:"roleArn" type:"string"`

	// The current status of the cluster.
	Status *string `locationName:"status" type:"string" enum:"ClusterStatus"`

	// Metadata that assists with categorization and organization. Each tag consists
	// of a key and an optional value. You define both. Tags don't propagate to
	// any other cluster or Amazon Web Services resources.
	Tags map[string]*string `locationName:"tags" min:"1" type:"map"`

	// The Kubernetes server version for the cluster.
	Version *string `locationName:"version" type:"string"`

	RemoteNetworkConfig *RemoteNetworkConfig `locationName:"remoteNetworkConfig" type:"structure"`
}

func CreateCluster(ctx context.Context, client *eks.EKS, input *CreateClusterInput, opts ...request.Option) (*CreateClusterOutput, error) {
	req, _ := client.CreateClusterRequest(&eks.CreateClusterInput{})
	req.Params = input
	out := &CreateClusterOutput{}
	req.Data = out
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}
