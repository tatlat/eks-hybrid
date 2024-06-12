package ssm

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/aws/eks-hybrid/internal/api"
)

const registrationFilePath = "/var/lib/amazon/ssm/registration"

type HybridInstanceRegistration struct {
	ManagedInstanceID string `json:"ManagedInstanceID"`
	Region            string `json:"Region"`
}

func (s *ssm) registerMachine(cfg *api.NodeConfig) error {
	registerCmd := exec.Command(InstallerPath, "-register", "-activation-code", cfg.Spec.Hybrid.SSM.ActivationCode,
		"-activation-id", cfg.Spec.Hybrid.SSM.ActivationID, "-region", cfg.Spec.Cluster.Region)
	return registerCmd.Run()
}

func GetManagedHybridInstanceId() (string, error) {
	data, err := os.ReadFile(registrationFilePath)
	if err != nil {
		return "", err
	}

	var registration HybridInstanceRegistration
	err = json.Unmarshal(data, &registration)
	if err != nil {
		return "", err
	}
	return registration.ManagedInstanceID, nil
}
