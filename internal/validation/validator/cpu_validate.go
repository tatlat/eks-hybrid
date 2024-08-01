package validator

import (
	"fmt"
	"runtime"

	"github.com/aws/eks-hybrid/internal/validation/logger"
)

const (
	minCPUCount         = 1
	recommendedCPUCount = 2

	prefixCPUCount            = "Number of CPUs ="
	prefixMinCPUCount         = "Minimum CPU Count ="
	prefixRecommendedCPUCount = "Recommended CPU Count ="
)

type NumCPU struct {
	GetCPUCount
}

type GetCPUCount func() int

func DefaultNumCPU() *NumCPU {
	return NewNumCPU(runtime.NumCPU)
}

func NewNumCPU(c GetCPUCount) *NumCPU {
	return &NumCPU{c}
}

func (v *NumCPU) Validate() error {
	num := v.GetCPUCount()
	info := fmt.Sprintf("%v | %v | %v", GetInfoStringInt(prefixCPUCount, num, ""), GetInfoStringInt(prefixMinCPUCount, minCPUCount, ""), GetInfoStringInt(prefixRecommendedCPUCount, recommendedCPUCount, ""))

	switch {
	case num < minCPUCount:
		logger.MarkFail(info)
		return &FailError{"Minimum CPU Count requirement not met"}
	case num < recommendedCPUCount:
		logger.MarkWarning(info)
		return &WarningError{"Recommended CPU Count requirement not met"}
	default:
		logger.MarkPass(info)
	}
	return nil
}
