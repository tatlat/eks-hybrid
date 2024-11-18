//go:build e2e
// +build e2e

package e2e

import (
	"context"
	_ "embed"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const rhelAWSAccount = "309956199498"

//go:embed testdata/rhel/8/cloud-init.txt
var rhel8CloudInit []byte

//go:embed testdata/rhel/9/cloud-init.txt
var rhel9CloudInit []byte

type rhelCloudInitData struct {
	NodeadmConfig     string
	NodeadmUrl        string
	KubernetesVersion string
	Provider          string
	RhelUsername      string
	RhelPassword      string
	RootPasswordHash  string
}

type RedHat8 struct {
	RhelUsername string
	RhelPassword string
	Architecture string
}

func NewRedHat8AMD(rhelUsername string, rhelPassword string) *RedHat8 {
	rh8 := new(RedHat8)
	rh8.RhelUsername = rhelUsername
	rh8.RhelPassword = rhelPassword
	rh8.Architecture = amd64Arch
	return rh8
}

func NewRedHat8ARM(rhelUsername string, rhelPassword string) *RedHat8 {
	rh8 := new(RedHat8)
	rh8.RhelUsername = rhelUsername
	rh8.RhelPassword = rhelPassword
	rh8.Architecture = arm64Arch
	return rh8
}

func (r RedHat8) Name() string {
	if r.Architecture == amd64Arch {
		return "rhel8-amd64"
	}
	return "rhel8-arm64"
}

func (r RedHat8) InstanceType() string {
	if r.Architecture == amd64Arch {
		return "m5.large"
	}
	return "t4g.large"
}

func (r RedHat8) AMIName(ctx context.Context, awsSession *session.Session) (string, error) {
	// there is no rhel ssm parameter
	// aws ec2 describe-images --owners 309956199498 --query 'sort_by(Images, &CreationDate)[-1].[ImageId]' --filters "Name=name,Values=RHEL-8*" "Name=architecture,Values=x86_64" --region us-west-2
	return findLatestImage(ec2.New(awsSession), "RHEL-8*", r.Architecture)
}

func (r RedHat8) BuildUserData(UserDataInput UserDataInput) ([]byte, error) {
	data := rhelCloudInitData{
		KubernetesVersion: UserDataInput.KubernetesVersion,
		NodeadmConfig:     UserDataInput.NodeadmConfigYaml,
		NodeadmUrl:        UserDataInput.NodeadmUrls.AMD,
		Provider:          UserDataInput.Provider,
		RhelUsername:      r.RhelUsername,
		RhelPassword:      r.RhelPassword,
		RootPasswordHash:  UserDataInput.RootPasswordHash,
	}

	if r.Architecture == arm64Arch {
		data.NodeadmUrl = UserDataInput.NodeadmUrls.ARM
	}

	return executeTemplate(rhel8CloudInit, data)
}

type RedHat9 struct {
	RhelUsername string
	RhelPassword string
	Architecture string
}

func NewRedHat9AMD(rhelUsername string, rhelPassword string) *RedHat9 {
	rh9 := new(RedHat9)
	rh9.RhelUsername = rhelUsername
	rh9.RhelPassword = rhelPassword
	rh9.Architecture = amd64Arch
	return rh9
}

func NewRedHat9ARM(rhelUsername string, rhelPassword string) *RedHat9 {
	rh9 := new(RedHat9)
	rh9.RhelUsername = rhelUsername
	rh9.RhelPassword = rhelPassword
	rh9.Architecture = arm64Arch
	return rh9
}
func (r RedHat9) Name() string {
	if r.Architecture == amd64Arch {
		return "rhel9-amd64"
	}
	return "rhel9-arm64"
}

func (r RedHat9) InstanceType() string {
	if r.Architecture == amd64Arch {
		return "m5.large"
	}
	return "t4g.large"
}

func (r RedHat9) AMIName(ctx context.Context, awsSession *session.Session) (string, error) {
	// there is no rhel ssm parameter
	// aws ec2 describe-images --owners 309956199498 --query 'sort_by(Images, &CreationDate)[-1].[ImageId]' --filters "Name=name,Values=RHEL-9*" "Name=architecture,Values=x86_64" --region us-west-2
	return findLatestImage(ec2.New(awsSession), "RHEL-9*", r.Architecture)
}

func (r RedHat9) BuildUserData(UserDataInput UserDataInput) ([]byte, error) {
	data := rhelCloudInitData{
		KubernetesVersion: UserDataInput.KubernetesVersion,
		NodeadmConfig:     UserDataInput.NodeadmConfigYaml,
		NodeadmUrl:        UserDataInput.NodeadmUrls.AMD,
		Provider:          UserDataInput.Provider,
		RhelUsername:      r.RhelUsername,
		RhelPassword:      r.RhelPassword,
		RootPasswordHash:  UserDataInput.RootPasswordHash,
	}

	if r.Architecture == arm64Arch {
		data.NodeadmUrl = UserDataInput.NodeadmUrls.ARM
	}

	return executeTemplate(rhel9CloudInit, data)
}

// AMI represents an ec2 Image.
type AMI struct {
	ID        string
	CreatedAt time.Time
}

// findLatestImage returns the most recent redhat image matching the amiPrefix and and arch
func findLatestImage(client *ec2.EC2, amiPrefix string, arch string) (string, error) {
	var latestAMI AMI

	in := &ec2.DescribeImagesInput{
		Owners:     []*string{aws.String(rhelAWSAccount)},
		Filters:    []*ec2.Filter{{Name: aws.String("name"), Values: []*string{aws.String(amiPrefix)}}, {Name: aws.String("architecture"), Values: []*string{aws.String(arch)}}},
		MaxResults: aws.Int64(100),
	}

	for {
		l, err := client.DescribeImages(in)
		if err != nil {
			return "", err
		}

		if paginationDone(in, l) {
			break
		}

		for _, i := range l.Images {
			created, err := time.Parse(time.RFC3339Nano, *i.CreationDate)
			if err != nil {
				return "", err
			}
			if created.Compare(latestAMI.CreatedAt) > 0 {
				latestAMI = AMI{
					ID:        *i.ImageId,
					CreatedAt: created,
				}
			}
		}

		in.NextToken = l.NextToken

		if in.NextToken == nil {
			break
		}
	}

	return latestAMI.ID, nil
}

func paginationDone(in *ec2.DescribeImagesInput, out *ec2.DescribeImagesOutput) bool {
	return (in.NextToken != nil && in.NextToken == out.NextToken) || len(out.Images) == 0
}
