package ssm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	awsSsm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const registrationFilePath = "/var/lib/amazon/ssm/registration"

type SSMRegistration struct {
	installRoot string
}

type SSMRegistrationOption func(*SSMRegistration)

func NewSSMRegistration(opts ...SSMRegistrationOption) *SSMRegistration {
	c := &SSMRegistration{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type SSMClient interface {
	DescribeInstanceInformation(ctx context.Context, params *awsSsm.DescribeInstanceInformationInput, optFns ...func(*awsSsm.Options)) (*awsSsm.DescribeInstanceInformationOutput, error)
	DeregisterManagedInstance(ctx context.Context, params *awsSsm.DeregisterManagedInstanceInput, optFns ...func(*awsSsm.Options)) (*awsSsm.DeregisterManagedInstanceOutput, error)
}

func WithInstallRoot(installRoot string) SSMRegistrationOption {
	return func(c *SSMRegistration) {
		c.installRoot = installRoot
	}
}

func Deregister(ctx context.Context, registration *SSMRegistration, ssmClient SSMClient, logger *zap.Logger) error {
	instanceId, err := registration.GetManagedHybridInstanceId()

	// If uninstall is being run just after running install and before running init
	// SSM would not be fully installed and registered, hence it's not required to run
	// deregister instance.
	if err != nil && os.IsNotExist(err) {
		logger.Info("Skipping SSM deregistration - node is not registered")
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "reading ssm registration file")
	}

	managed, err := isInstanceManaged(ssmClient, instanceId)
	if err != nil {
		return errors.Wrapf(err, "getting managed instance information")
	}

	// Only deregister the instance if init/ssm init was run and
	// if instances is actively listed as managed
	if managed {
		if err := deregister(ssmClient, instanceId); err != nil {
			return errors.Wrapf(err, "deregistering ssm managed instance")
		}
	}
	return nil
}

func (r *SSMRegistration) getManagedHybridInstanceIdAndRegion() (string, string, error) {
	data, err := os.ReadFile(r.RegistrationFilePath())
	if err != nil {
		return "", "", err
	}

	var registration HybridInstanceRegistration
	err = json.Unmarshal(data, &registration)
	if err != nil {
		return "", "", err
	}
	return registration.ManagedInstanceID, registration.Region, nil
}

// GetRegion returns the region of the managed hybrid instance
// If the instance is not registered, it returns an empty string
// errors are ignored and an empty string is returned
func (r *SSMRegistration) GetRegion() string {
	registered, err := r.isRegistered()
	if err != nil || !registered {
		return ""
	}
	_, region, err := r.getManagedHybridInstanceIdAndRegion()
	if err != nil {
		return ""
	}
	return region
}

func (r *SSMRegistration) GetManagedHybridInstanceId() (string, error) {
	data, err := os.ReadFile(r.RegistrationFilePath())
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

func (r *SSMRegistration) isRegistered() (bool, error) {
	_, err := r.GetManagedHybridInstanceId()
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading ssm registration file: %w", err)
	}
	return true, nil
}

// RegistrationFilePath returns the path to the SSM registration file
// If installRoot is not set, it will return the path starting from the disk root
func (r *SSMRegistration) RegistrationFilePath() string {
	return filepath.Join(r.installRoot, registrationFilePath)
}
