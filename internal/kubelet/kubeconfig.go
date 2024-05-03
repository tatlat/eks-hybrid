package kubelet

import (
	"bytes"
	_ "embed"
	"path"
	"text/template"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/util"
)

const (
	kubeconfigRoot          = "/var/lib/kubelet"
	kubeconfigFile          = "kubeconfig"
	kubeconfigBootstrapFile = "bootstrap-kubeconfig"
	kubeconfigPerm          = 0644
)

var (
	//go:embed kubeconfig.template.yaml
	kubeconfigTemplateData string
	//go:embed hybrid-kubeconfig.template.yaml
	hybridKubeconfigTemplateData string
	kubeconfigPath               = path.Join(kubeconfigRoot, kubeconfigFile)
	kubeconfigBootstrapPath      = path.Join(kubeconfigRoot, kubeconfigBootstrapFile)
)

func (k *kubelet) writeKubeconfig(cfg *api.NodeConfig) error {
	kubeconfig, err := generateKubeconfig(cfg)
	if err != nil {
		return err
	}
	if cfg.IsOutpostNode() {
		// kubelet bootstrap kubeconfig uses aws-iam-authenticator with cluster id to authenticate to cluster
		//   - if "aws eks describe-cluster" is bypassed, for local outpost, the value of CLUSTER_NAME parameter will be cluster id.
		//   - otherwise, the cluster id will use the id returned by "aws eks describe-cluster".
		k.flags["bootstrap-kubeconfig"] = kubeconfigBootstrapPath
		return util.WriteFileWithDir(kubeconfigBootstrapPath, kubeconfig, kubeconfigPerm)
	} else {
		k.flags["kubeconfig"] = kubeconfigPath
		return util.WriteFileWithDir(kubeconfigPath, kubeconfig, kubeconfigPerm)
	}
}

type kubeconfigTemplateVars struct {
	Cluster           string
	Region            string
	APIServerEndpoint string
	CaCertPath        string
	SessionName       string
	AssumeRole        string
}

func newKubeconfigTemplateVars(cfg *api.NodeConfig) *kubeconfigTemplateVars {
	return &kubeconfigTemplateVars{
		Cluster:           cfg.Spec.Cluster.Name,
		Region:            cfg.Status.Instance.Region,
		APIServerEndpoint: cfg.Spec.Cluster.APIServerEndpoint,
		CaCertPath:        caCertificatePath,
	}
}

func (kct *kubeconfigTemplateVars) withOutpostVars(cfg *api.NodeConfig) {
	kct.Cluster = cfg.Spec.Cluster.ID
}

func (kct *kubeconfigTemplateVars) withHybridVars(cfg *api.NodeConfig) {
	kct.Region = cfg.Spec.Hybrid.Region
	kct.SessionName = cfg.Spec.Hybrid.NodeName
	kct.AssumeRole = cfg.Spec.Hybrid.IAMRolesAnywhere.RoleARN
}

func generateKubeconfig(cfg *api.NodeConfig) ([]byte, error) {
	config := newKubeconfigTemplateVars(cfg)
	if cfg.IsOutpostNode() {
		config.withOutpostVars(cfg)
	}
	if cfg.IsHybridNode() {
		config.withHybridVars(cfg)
	}

	var buf bytes.Buffer
	var kubeconfigTemplate *template.Template
	if cfg.IsHybridNode() {
		kubeconfigTemplate = template.Must(template.New(kubeconfigFile).Parse(hybridKubeconfigTemplateData))
	} else {
		kubeconfigTemplate = template.Must(template.New(kubeconfigFile).Parse(kubeconfigTemplateData))
	}
	if err := kubeconfigTemplate.Execute(&buf, config); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
