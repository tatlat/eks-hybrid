package hybrid

import (
	"fmt"

	"github.com/aws/eks-hybrid/internal/api"
)

func (hnp *hybridNodeProvider) withHybridValidators() {
	hnp.validator = func(cfg *api.NodeConfig) error {
		if cfg.Spec.Cluster.Name == "" {
			return fmt.Errorf("Name is missing in cluster configuration")
		}
		if cfg.Spec.Cluster.Region == "" {
			return fmt.Errorf("Region is missing in cluster configuration")
		}
		if !cfg.IsIAMRolesAnywhere() && !cfg.IsSSM() {
			return fmt.Errorf("Either IAMRolesAnywhere or SSM must be provided for hybrid node configuration")
		}
		if cfg.IsIAMRolesAnywhere() && cfg.IsSSM() {
			return fmt.Errorf("Only one of IAMRolesAnywhere or SSM must be provided for hybrid node configuration")
		}
		if cfg.IsIAMRolesAnywhere() {
			if cfg.Spec.Hybrid.IAMRolesAnywhere.RoleARN == "" {
				return fmt.Errorf("RoleARN is missing in hybrid iam roles anywhere configuration")
			}
			if cfg.Spec.Hybrid.IAMRolesAnywhere.ProfileARN == "" {
				return fmt.Errorf("ProfileARN is missing in hybrid iam roles anywhere configuration")
			}
			if cfg.Spec.Hybrid.IAMRolesAnywhere.TrustAnchorARN == "" {
				return fmt.Errorf("TrustAnchorARN is missing in hybrid iam roles anywhere configuration")
			}
			if cfg.Spec.Hybrid.IAMRolesAnywhere.NodeName == "" {
				return fmt.Errorf("NodeName can't be empty in hybrid iam roles anywhere configuration")
			}
			if len(cfg.Spec.Hybrid.IAMRolesAnywhere.NodeName) > 64 {
				return fmt.Errorf("NodeName can't be longer than 64 characters in hybrid iam roles anywhere configuration")
			}
		}
		if cfg.IsSSM() {
			if cfg.Spec.Hybrid.SSM.ActivationCode == "" {
				return fmt.Errorf("ActivationCode is missing in hybrid ssm configuration")
			}
			if cfg.Spec.Hybrid.SSM.ActivationID == "" {
				return fmt.Errorf("ActivationID is missing in hybrid ssm configuration")
			}
		}
		return nil
	}
}

func (hnp *hybridNodeProvider) ValidateConfig() error {
	hnp.logger.Info("Validating configuration...")
	if err := hnp.validator(hnp.nodeConfig); err != nil {
		return err
	}
	return nil
}
