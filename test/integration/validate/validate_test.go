//go:build e2e

package validate

import (
	"os"
	"testing"
)

const (
	region            = "us-west-2"
	nodeadmE2ERoleEnv = "NODEADM_E2E_ROLE"

	ubuntuImageId = "ami-0aff18ec83b712f05"
	rhelImageId   = "ami-05b40ce1c0e236ef2"
	suseImageId   = "ami-0f0fb8a9b56690cf3"

	amd64 = "amd64"
	arm64 = "arm64"

	passVolumeSize = int64(30)
	failVolumeSize = int64(10)

	passUbuntuEC2Type                = "t3.large"
	failNotEnoughMemoryUbuntuEC2Type = "t3.micro"
	failNotEnoughCPUUbuntuEC2Type    = "t3.small"
	passRhelEC2Type                  = "t4g.xlarge"
	failNotEnoughMemoryRhelEC2Type   = "t4g.micro"
	failNotEnoughCPURhelEC2Type      = "c6g.medium"
	failSuseEC2Type                  = "t3.large"

	ubuntuUserData = `#!/bin/bash
	echo "ssh_pwauth: false" >> /etc/cloud/cloud.cfg`
	rhelUserData = `#!/bin/bash
	sudo yum update
	sudo yum install -y https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/linux_arm64/amazon-ssm-agent.rpm
	sudo systemctl start amazon-ssm-agent
	sudo systemctl enable amazon-ssm-agent
	sudo systemctl status amazon-ssm-agent
	echo "ssh_pwauth: false" >> /etc/cloud/cloud.cfg`
	suseUserData = `#!/bin/bash
	sudo zypper refresh
	sudo zypper install -y curl tar gzip
	mkdir /tmp/ssm
	cd /tmp/ssm
	wget https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/linux_amd64/amazon-ssm-agent.rpm
	sudo rpm --install amazon-ssm-agent.rpm
	sudo systemctl start amazon-ssm-agent
	sudo systemctl enable amazon-ssm-agent
	sudo systemctl status amazon-ssm-agent
	echo "ssh_pwauth: false" >> /etc/cloud/cloud.cfg`
)

func TestValidate(t *testing.T) {
	for _, test := range []struct {
		name        string
		instance    EC2Instance
		errExpected bool
	}{
		{
			name: "test-ubuntu-success",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           ubuntuImageId,
				ec2Type:           passUbuntuEC2Type,
				volumeSize:        passVolumeSize,
				arch:              amd64,
				userData:          ubuntuUserData,
			},
			errExpected: false,
		},
		{
			name: "test-ubuntu-fail-not-enough-disk-space",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           ubuntuImageId,
				ec2Type:           passUbuntuEC2Type,
				volumeSize:        failVolumeSize,
				arch:              amd64,
				userData:          ubuntuUserData,
			},
			errExpected: true,
		},
		{
			name: "test-ubuntu-fail-not-enough-memory",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           ubuntuImageId,
				ec2Type:           failNotEnoughMemoryUbuntuEC2Type,
				volumeSize:        passVolumeSize,
				arch:              amd64,
				userData:          ubuntuUserData,
			},
			errExpected: true,
		},
		{
			name: "test-ubuntu-fail-not-enough-cpu",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           ubuntuImageId,
				ec2Type:           failNotEnoughCPUUbuntuEC2Type,
				volumeSize:        passVolumeSize,
				arch:              amd64,
				userData:          ubuntuUserData,
			},
			errExpected: true,
		},
		{
			name: "test-rhel-success",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           rhelImageId,
				ec2Type:           passRhelEC2Type,
				volumeSize:        passVolumeSize,
				arch:              arm64,
				userData:          rhelUserData,
			},
			errExpected: false,
		},
		{
			name: "test-rhel-fail-not-enough-disk-space",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           rhelImageId,
				ec2Type:           passRhelEC2Type,
				volumeSize:        failVolumeSize,
				arch:              arm64,
				userData:          rhelUserData,
			},
			errExpected: true,
		},
		{
			name: "test-rhel-fail-not-enough-memory",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           rhelImageId,
				ec2Type:           failNotEnoughMemoryRhelEC2Type,
				volumeSize:        passVolumeSize,
				arch:              arm64,
				userData:          rhelUserData,
			},
			errExpected: true,
		},
		{
			name: "test-rhel-fail-not-enough-cpu",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           rhelImageId,
				ec2Type:           failNotEnoughCPURhelEC2Type,
				volumeSize:        passVolumeSize,
				arch:              arm64,
				userData:          rhelUserData,
			},
			errExpected: true,
		},
		{
			name: "test-suse-os-not-supported",
			instance: EC2Instance{
				nodeadmE2ERoleEnv: os.Getenv(nodeadmE2ERoleEnv),
				imageID:           suseImageId,
				ec2Type:           failSuseEC2Type,
				volumeSize:        passVolumeSize,
				arch:              amd64,
				userData:          suseUserData,
			},
			errExpected: true,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runner, err := SetUpTest(region, test.instance, test.name)
			if err != nil {
				t.Errorf("Failed to set up test: %v", err)
			}

			errs := runner.Run()
			if errs != nil && !test.errExpected {
				for _, e := range errs {
					t.Errorf("Failed to run test: %v", e)
				}
			}

			err = runner.DeleteInstance()
			if err != nil {
				t.Errorf("Failed to terminate instance: %v", err)
			}

		})
	}
}
