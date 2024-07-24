package system

import (
	"fmt"
	"github.com/aws/eks-hybrid/internal/api"
	"os/exec"
)

const swapAspectName = "swap"

type swapAspect struct{}

var _ SystemAspect = &swapAspect{}

func NewSwapAspect() *swapAspect {
	return &swapAspect{}
}

func (s *swapAspect) Name() string {
	return swapAspectName
}

func (s *swapAspect) Setup(cfg *api.NodeConfig) error {
	return swapOff()
}

func swapOff() error {
	offCmd := exec.Command("swapoff", "--all")
	out, err := offCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to turn off swap: %s, error: %v", out, err)
	}
	return nil
}
