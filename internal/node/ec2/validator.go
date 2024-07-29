package ec2

import (
	"fmt"

	"github.com/aws/eks-hybrid/internal/api"
)

func (enp *ec2NodeProvider) withEc2NodeValidators() {
	enp.validator = func(cfg *api.NodeConfig) error {
		if cfg.Spec.Cluster.Name == "" {
			return fmt.Errorf("Name is missing in cluster configuration")
		}
		if cfg.Spec.Cluster.APIServerEndpoint == "" {
			return fmt.Errorf("Apiserver endpoint is missing in cluster configuration")
		}
		if cfg.Spec.Cluster.CertificateAuthority == nil {
			return fmt.Errorf("Certificate authority is missing in cluster configuration")
		}
		if cfg.Spec.Cluster.CIDR == "" {
			return fmt.Errorf("CIDR is missing in cluster configuration")
		}
		if cfg.IsOutpostNode() {
			if cfg.Spec.Cluster.ID == "" {
				return fmt.Errorf("CIDR is missing in cluster configuration")
			}
		}
		return nil
	}
}

func (enp *ec2NodeProvider) ValidateConfig() error {
	if err := enp.validator(enp.nodeConfig); err != nil {
		return err
	}
	return nil
}
