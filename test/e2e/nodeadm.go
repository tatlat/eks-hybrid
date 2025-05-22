package e2e

import (
	"context"
	_ "embed"

	"github.com/aws/aws-sdk-go-v2/aws"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/creds"
)

// NodeadmOS defines an interface for operating system-specific behavior.
type NodeadmOS interface {
	Name() string
	AMIName(ctx context.Context, awsConfig aws.Config) (string, error)
	BuildUserData(userDataInput UserDataInput) ([]byte, error)
	InstanceType(region string, instanceSize InstanceSize) string
}

type InstanceSize int

const (
	Small InstanceSize = iota
	Medium
	Large
	XLarge
)

type UserDataInput struct {
	CredsProviderName string
	EKSEndpoint       string
	KubernetesVersion string
	NodeadmUrls       NodeadmURLs
	NodeadmConfigYaml string
	Provider          string
	PublicKey         string
	Region            string
	RootPasswordHash  string
	Files             []File
}

type NodeadmURLs struct {
	AMD string
	ARM string
}

type LogsUploadUrl struct {
	Name string
	Url  string
}

// File represents a file in disk.
type File struct {
	Content     string
	Path        string
	Permissions string
}

type NodeadmCredentialsProvider interface {
	Name() creds.CredentialProvider
	NodeadmConfig(ctx context.Context, node NodeSpec) (*api.NodeConfig, error)
	VerifyUninstall(ctx context.Context, instanceId string) error
	FilesForNode(spec NodeSpec) ([]File, error)
}

// HybridEC2Node represents a Hybrid Node backed by an EC2 instance.
type HybridEC2Node struct {
	Node corev1.Node
}

// NodeSpec is a specification for a node.
type NodeSpec struct {
	Cluster  *Cluster
	Name     string
	OS       CredsOS
	Provider NodeadmCredentialsProvider
}

type Cluster struct {
	Name   string
	Region string
}

// CredsOS is the Node OS.
type CredsOS interface {
	Name() string
}
