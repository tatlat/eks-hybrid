package iamrolesanywhere

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/util"
)

const (
	DaemonName                   = "aws_signing_helper_update"
	EksHybridAwsCredentialsPath  = "/eks-hybrid/.aws/credentials"
	SigningHelperServiceFilePath = "/etc/systemd/system/aws_signing_helper_update.service"
)

var (
	//go:embed aws_signing_helper_update_service.tpl
	rawSigningHelperServiceTemplate string

	signingHelperServiceTemplate = template.Must(template.New("").Parse(rawSigningHelperServiceTemplate))
)

type SigningHelperDaemon struct {
	daemonManager daemon.DaemonManager
	spec          *api.NodeConfigSpec
}

func NewSigningHelperDaemon(daemonManager daemon.DaemonManager, spec *api.NodeConfigSpec) daemon.Daemon {
	return &SigningHelperDaemon{
		daemonManager: daemonManager,
		spec:          spec,
	}
}

func (s *SigningHelperDaemon) Configure() error {
	var buf bytes.Buffer
	if err := signingHelperServiceTemplate.Execute(&buf, map[string]string{
		"SharedCredentialsFilePath": EksHybridAwsCredentialsPath,
		"SigningHelperBinPath":      SigningHelperBinPath,
		"TrustAnchorARN":            s.spec.Hybrid.IAMRolesAnywhere.TrustAnchorARN,
		"ProfileARN":                s.spec.Hybrid.IAMRolesAnywhere.ProfileARN,
		"RoleARN":                   s.spec.Hybrid.IAMRolesAnywhere.RoleARN,
		"Region":                    s.spec.Cluster.Region,
		"NodeName":                  s.spec.Hybrid.IAMRolesAnywhere.NodeName,
	}); err != nil {
		return fmt.Errorf("executing aws_signing_helper_update service template: %v", err)
	}

	if err := util.WriteFileWithDir(SigningHelperServiceFilePath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing aws_signing_helper_update service file %s: %v", EksHybridAwsCredentialsPath, err)
	}
	return nil
}

// EnsureRunning enables and starts the aws_signing_helper unit.
func (s *SigningHelperDaemon) EnsureRunning(ctx context.Context) error {
	err := s.daemonManager.EnableDaemon(s.Name())
	if err != nil {
		return err
	}
	return s.daemonManager.RestartDaemon(ctx, s.Name())
}

// PostLaunch runs any additional step that needs to occur after the service
// daemon as been started.
func (s *SigningHelperDaemon) PostLaunch() error {
	return nil
}

// Stop stops the aws_signing_helper unit only if it is loaded and running.
func (s *SigningHelperDaemon) Stop() error {
	return s.daemonManager.StopDaemon(s.Name())
}

// Name returns the name of the daemon.
func (s *SigningHelperDaemon) Name() string {
	return DaemonName
}
