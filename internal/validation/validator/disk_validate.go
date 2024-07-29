package validator

import (
	"fmt"
	"os"

	"github.com/aws/eks-hybrid/internal/validation/logger"
	"golang.org/x/sys/unix"
)

const (
	minDisk float64 = float64(20)

	prefixDisk    = "Available disk size ="
	prefixMinDisk = "Minimum Disk Size ="
	unitDisk      = "GB"
)

type DiskSize struct {
	GetDiskSize
}

type GetDiskSize func() (float64, error)

func DefaultDiskSize() *DiskSize {
	return NewDiskSize(getDiskSize)
}

func NewDiskSize(d GetDiskSize) *DiskSize {
	return &DiskSize{d}
}

func (v *DiskSize) Validate() error {
	size, err := v.GetDiskSize()
	if err != nil {
		logger.Error(err, "Error getting disk size")
		return err
	}

	info := fmt.Sprintf("%v | %v ", GetInfoStringFloat64(prefixDisk, size, unitDisk), GetInfoStringFloat64(prefixMinDisk, minDisk, unitDisk))

	switch {
	case size < minDisk:
		logger.MarkWarning(info)
		return &WarningError{"Minimum disk size requirement not met"}
	default:
		logger.MarkPass(info)
	}
	return nil
}

func getDiskSize() (float64, error) {
	var stat unix.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("error getting disk size: %v", err)
	}
	err = unix.Statfs(wd, &stat)
	if err != nil {
		return 0, fmt.Errorf("error getting disk size: %v", err)
	}
	return BToGb(stat.Bavail * uint64(stat.Bsize)), nil
}
