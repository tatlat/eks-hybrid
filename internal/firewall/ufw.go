package firewall

import (
	"fmt"
	"os/exec"
	"regexp"
)

const ufwBinary = "ufw"

var ufwActiveRegex = regexp.MustCompile(`.*Status: active*`)

type UncomplicatedFireWall struct {
	binPath string
}

func NewUncomplicatedFirewall() Manager {
	path, _ := exec.LookPath(ufwBinary)
	return &UncomplicatedFireWall{
		binPath: path,
	}
}

// IsEnabled returns true if ufw is enabled and running on the node
func (ufw *UncomplicatedFireWall) IsEnabled() (bool, error) {
	// Check if ufw is installed
	if ufw.binPath != "" {
		statusCmd := exec.Command(ufw.binPath, "status")
		out, err := statusCmd.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("failed to get status of uncomplicated firewall: %s, error: %v", out, err)
		}
		// Check for active status output
		if match := ufwActiveRegex.MatchString(string(out)); match {
			return true, nil
		}
	}
	return false, nil
}

// AllowTcpPort adds a rule to the firewall to open input port
func (ufw *UncomplicatedFireWall) AllowTcpPort(port string) error {
	portAddCmd := exec.Command(ufw.binPath, "allow", fmt.Sprintf("%s/tcp", port))
	out, err := portAddCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to allow port %s in firewall: %s, error: %v", port, out, err)
	}
	return nil
}

// AllowTcpPortRange adds a rule to the firewall to open the range of input port
func (ufw *UncomplicatedFireWall) AllowTcpPortRange(startPort, endPort string) error {
	portAddCmd := exec.Command(ufw.binPath, "allow", fmt.Sprintf("%s:%s/tcp", startPort, endPort))
	out, err := portAddCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to allow ports in firewall: %s, error: %v", out, err)
	}
	return nil
}

// FlushRules flushes the rules and reloads the firewall to enforce the rules
func (ufw *UncomplicatedFireWall) FlushRules() error {
	// UFW activates the rules the moment its added, there is no need to flush them out to disk explicitly
	return nil
}
