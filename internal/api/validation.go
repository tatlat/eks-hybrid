package api

import "fmt"

func ValidateNodeConfig(cfg *NodeConfig) error {
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
	if enabled := cfg.Spec.Cluster.EnableOutpost; enabled != nil && *enabled {
		if cfg.Spec.Cluster.ID == "" {
			return fmt.Errorf("CIDR is missing in cluster configuration")
		}
	}
	// Validate all hybrid node configuration
	if cfg.IsHybridNode() {
		if cfg.Spec.Hybrid.Region == "" {
			return fmt.Errorf("Region is missing in hybrid configuration")
		}
		if cfg.Spec.Hybrid.IAMRolesAnywhere == nil && cfg.Spec.Hybrid.SSM == nil {
			return fmt.Errorf("Either IAMRolesAnywhere or SSM must be provided for hybrid node configuration")
		}
		if cfg.Spec.Hybrid.IAMRolesAnywhere != nil && cfg.Spec.Hybrid.SSM != nil {
			return fmt.Errorf("Only one of IAMRolesAnywhere or SSM must be provided for hybrid node configuration")
		}
	}
	return nil
}
