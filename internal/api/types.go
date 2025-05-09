// +kubebuilder:object:generate=true
// +groupName=node.eks.aws
package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:skipversion
// +kubebuilder:object:root=true

type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeConfigSpec `json:"spec,omitempty"`
	// +k8s:conversion-gen=false
	Status NodeConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NodeConfig `json:"items"`
}

type NodeConfigSpec struct {
	Cluster    ClusterDetails    `json:"cluster,omitempty"`
	Containerd ContainerdOptions `json:"containerd,omitempty"`
	Instance   InstanceOptions   `json:"instance,omitempty"`
	Kubelet    KubeletOptions    `json:"kubelet,omitempty"`
	Hybrid     *HybridOptions    `json:"hybrid,omitempty"`
}

type NodeConfigStatus struct {
	Instance InstanceDetails `json:"instance,omitempty"`
	Hybrid   HybridDetails   `json:"hybrid,omitempty"`
	Defaults DefaultOptions  `json:"default,omitempty"`
}

type InstanceDetails struct {
	ID               string `json:"id,omitempty"`
	Region           string `json:"region,omitempty"`
	Type             string `json:"type,omitempty"`
	AvailabilityZone string `json:"availabilityZone,omitempty"`
	MAC              string `json:"mac,omitempty"`
	PrivateDNSName   string `json:"privateDnsName,omitempty"`
}

type HybridDetails struct {
	NodeName string `json:"nodeName,omitempty"`
}

type DefaultOptions struct {
	SandboxImage string `json:"sandboxImage,omitempty"`
}

type ClusterDetails struct {
	Name                 string `json:"name,omitempty"`
	Region               string `json:"region,omitempty"`
	APIServerEndpoint    string `json:"apiServerEndpoint,omitempty"`
	CertificateAuthority []byte `json:"certificateAuthority,omitempty"`
	CIDR                 string `json:"cidr,omitempty"`
	EnableOutpost        *bool  `json:"enableOutpost,omitempty"`
	ID                   string `json:"id,omitempty"`
}

type KubeletOptions struct {
	// Config is a kubelet config that can be provided by the user to override
	// default generated configurations
	// https://kubernetes.io/docs/reference/config-api/kubelet-config.v1/
	Config InlineDocument `json:"config,omitempty"`
	// Flags is a list of command-line kubelet arguments. These arguments are
	// amended to the generated defaults, and therefore will act as overrides
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/
	Flags []string `json:"flags,omitempty"`
}

// InlineDocument is an alias to a dynamically typed map. This allows using
// embedded YAML and JSON types within the parent yaml config.
type InlineDocument map[string]runtime.RawExtension

type ContainerdOptions struct {
	// Config is an inline containerd config toml document that can be provided
	// by the user to override default generated configurations
	// https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md
	Config string `json:"config,omitempty"`
}

type IPFamily string

const (
	IPFamilyIPv4 IPFamily = "ipv4"
	IPFamilyIPv6 IPFamily = "ipv6"
)

type InstanceOptions struct {
	LocalStorage LocalStorageOptions `json:"localStorage,omitempty"`
}

type LocalStorageOptions struct {
	Strategy LocalStorageStrategy `json:"strategy,omitempty"`
}

type LocalStorageStrategy string

const (
	LocalStorageRAID0 LocalStorageStrategy = "RAID0"
	LocalStorageMount LocalStorageStrategy = "Mount"
)

type NodeType string

const (
	Ssm              NodeType = "ssm"
	IamRolesAnywhere NodeType = "iam-ra"
	Ec2              NodeType = "ec2"
	Outpost          NodeType = "outpost"
)

type HybridOptions struct {
	EnableCredentialsFile bool              `json:"enableCredentialsFile,omitempty"`
	IAMRolesAnywhere      *IAMRolesAnywhere `json:"iamRolesAnywhere,omitempty"`
	SSM                   *SSM              `json:"ssm,omitempty"`
}

func (nc NodeConfig) IsHybridNode() bool {
	return nc.Spec.Hybrid != nil
}

func (nc NodeConfig) IsOutpostNode() bool {
	enabled := nc.Spec.Cluster.EnableOutpost
	return enabled != nil && *enabled
}

func (nc NodeConfig) IsIAMRolesAnywhere() bool {
	return nc.Spec.Hybrid != nil && nc.Spec.Hybrid.IAMRolesAnywhere != nil
}

func (nc NodeConfig) IsSSM() bool {
	return nc.Spec.Hybrid != nil && nc.Spec.Hybrid.SSM != nil
}

func (nc NodeConfig) GetNodeType() NodeType {
	if nc.IsSSM() {
		return Ssm
	} else if nc.IsIAMRolesAnywhere() {
		return IamRolesAnywhere
	} else if nc.IsOutpostNode() {
		return Outpost
	}
	return Ec2
}

type IAMRolesAnywhere struct {
	NodeName        string `json:"nodeName,omitempty"`
	TrustAnchorARN  string `json:"trustAnchorArn,omitempty"`
	ProfileARN      string `json:"profileArn,omitempty"`
	RoleARN         string `json:"roleArn,omitempty"`
	AwsConfigPath   string `json:"awsConfigPath,omitempty"`
	CertificatePath string `json:"certificatePath,omitempty"`
	PrivateKeyPath  string `json:"privateKeyPath,omitempty"`
}

type SSM struct {
	ActivationCode string `json:"activationCode,omitempty"`
	ActivationID   string `json:"activationId,omitempty"`
}
