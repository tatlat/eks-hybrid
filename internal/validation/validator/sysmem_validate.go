package validator

import (
	"fmt"

	"github.com/aws/eks-hybrid/internal/validation/logger"
	"github.com/pbnjay/memory"
)

const (
	minMemory         = float64(2)
	recommendedMemory = float64(4)

	prefixMemory            = "Sys Memory ="
	prefixMinMemory         = "Minimum Sys Memory ="
	prefixRecommendedMemory = "Recommended Sys Memory ="
	unitMemory              = "GB"
)

type SysMem struct {
	GetMemCount
}

type GetMemCount func() uint64

func DefaultSysMem() *SysMem {
	return NewSysMem(memory.TotalMemory)
}

func NewSysMem(m GetMemCount) *SysMem {
	return &SysMem{m}
}

func (v *SysMem) Validate() error {
	memory := BToGb(v.GetMemCount())
	info := fmt.Sprintf("%v | %v | %v", GetInfoStringFloat64(prefixMemory, memory, unitMemory), GetInfoStringFloat64(prefixMinMemory, minMemory, unitMemory), GetInfoStringFloat64(prefixRecommendedMemory, recommendedMemory, unitMemory))

	switch {
	case memory < minMemory:
		logger.MarkFail(info)
		return &FailError{"Minimum System memory requirement not met"}
	case memory < recommendedMemory:
		logger.MarkWarning(info)
		return &WarningError{"Recommended System memory requirement not met"}
	default:
		logger.MarkPass(info)
	}
	return nil
}
