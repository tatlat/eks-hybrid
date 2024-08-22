package system

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/eks-hybrid/internal/api"
)

const swapAspectName = "swap"

type swapAspect struct {
	nodeConfig *api.NodeConfig
}

var _ SystemAspect = &swapAspect{}

func NewSwapAspect(cfg *api.NodeConfig) SystemAspect {
	return &swapAspect{nodeConfig: cfg}
}

func (s *swapAspect) Name() string {
	return swapAspectName
}

func (s *swapAspect) Setup() error {
	if err := swapOff(); err != nil {
		return err
	}
	return disableSwapOnFstab()
}

func swapOff() error {
	offCmd := exec.Command("swapoff", "--all")
	out, err := offCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to turn off swap: %s, error: %v", out, err)
	}
	return nil
}

// Mount represets the filesystem info
type mount struct {
	// The block special device or remote filesystem to be mounted
	spec string

	// The mount point for the filesystem
	file string

	// The type of the filesystem
	vfsType string
}

func disableSwapOnFstab() error {
	file, err := os.OpenFile("/etc/fstab", os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	var bs []byte
	buf := bytes.NewBuffer(bs)
	scanner := bufio.NewScanner(file)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		fstabMount, err := parseFstabLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("/etc/fstab syntax error at line %d: %s", lineNo, err)
		}
		if fstabMount == nil || fstabMount != nil && fstabMount.vfsType != "swap" {
			buf.WriteString(scanner.Text() + "\n")
		}
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	_, err = buf.WriteTo(file)
	return err
}

func parseFstabLine(line string) (*mount, error) {
	line = strings.TrimSpace(line)

	// Lines starting with a pound sign (#) are comments, and are ignored. So are empty lines.
	if (line == "") || (line[0] == '#') {
		return nil, nil
	}

	fields := strings.Fields(line)
	fstabMount := &mount{}
	if len(fields) < 3 {
		return nil, fmt.Errorf("too few fields (%d), at least 4 are expected", len(fields))
	} else {
		fstabMount.spec = fields[0]
		fstabMount.file = fields[1]
		fstabMount.vfsType = fields[2]
	}

	return fstabMount, nil
}
