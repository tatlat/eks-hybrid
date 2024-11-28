//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

//go:embed testdata/ubuntu/2004/cloud-init.txt
var ubuntu2004CloudInit []byte

//go:embed testdata/ubuntu/2204/cloud-init.txt
var ubuntu2204CloudInit []byte

//go:embed testdata/ubuntu/2404/cloud-init.txt
var ubuntu2404CloudInit []byte

//go:embed testdata/nodeadm-init.sh
var nodeAdmInitScript []byte

//go:embed testdata/log-collector.sh
var logCollectorScript []byte

type ubuntuCloudInitData struct {
	UserDataInput
	NodeadmUrl            string
	NodeadmInitScript     string
	NodeadmAdditionalArgs string
}

func templateFuncMap() map[string]interface{} {
	return map[string]interface{}{
		"indent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return pad + strings.Replace(v, "\n", "\n"+pad, -1)
		},
	}
}

type Ubuntu2004 struct {
	Architecture     string
	ContainerdSource string
}

func NewUbuntu2004AMD() *Ubuntu2004 {
	u := new(Ubuntu2004)
	u.Architecture = "amd64"
	u.ContainerdSource = "distro"
	return u
}

func NewUbuntu2004DockerSource() *Ubuntu2004 {
	u := new(Ubuntu2004)
	u.Architecture = "amd64"
	u.ContainerdSource = "docker"
	return u
}

func NewUbuntu2004ARM() *Ubuntu2004 {
	u := new(Ubuntu2004)
	u.Architecture = arm64Arch
	u.ContainerdSource = "distro"
	return u
}

func (u Ubuntu2004) Name() string {
	name := "ubuntu2004-" + u.Architecture
	if u.ContainerdSource == "docker" {
		name += "-docker"
	}
	return name
}

func (u Ubuntu2004) InstanceType() string {
	if u.Architecture == "amd64" {
		return "m5.2xlarge"
	}
	return "t4g.2xlarge"
}

func (u Ubuntu2004) AMIName(ctx context.Context, awsSession *session.Session) (string, error) {
	amiId, err := getAmiIDFromSSM(ctx, ssm.New(awsSession), "/aws/service/canonical/ubuntu/server/20.04/stable/current/"+u.Architecture+"/hvm/ebs-gp2/ami-id")
	return *amiId, err
}

func (u Ubuntu2004) BuildUserData(userDataInput UserDataInput) ([]byte, error) {
	if err := populateBaseScripts(&userDataInput); err != nil {
		return nil, err
	}

	data := ubuntuCloudInitData{
		UserDataInput: userDataInput,
		NodeadmUrl:    userDataInput.NodeadmUrls.AMD,
	}

	if u.Architecture == arm64Arch {
		data.NodeadmUrl = userDataInput.NodeadmUrls.ARM
	}

	if u.ContainerdSource == "docker" {
		data.NodeadmAdditionalArgs = "--containerd-source docker"
	}

	return executeTemplate(ubuntu2004CloudInit, data)
}

type Ubuntu2204 struct {
	Architecture     string
	ContainerdSource string
}

func NewUbuntu2204AMD() *Ubuntu2204 {
	u := new(Ubuntu2204)
	u.Architecture = "amd64"
	u.ContainerdSource = "distro"
	return u
}
func NewUbuntu2204DockerSource() *Ubuntu2204 {
	u := new(Ubuntu2204)
	u.Architecture = "amd64"
	u.ContainerdSource = "docker"
	return u
}

func NewUbuntu2204ARM() *Ubuntu2204 {
	u := new(Ubuntu2204)
	u.Architecture = arm64Arch
	u.ContainerdSource = "distro"
	return u
}

func (u Ubuntu2204) Name() string {
	name := "ubuntu2204-" + u.Architecture
	if u.ContainerdSource == "docker" {
		name += "-docker"
	}
	return name
}

func (u Ubuntu2204) InstanceType() string {
	if u.Architecture == "amd64" {
		return "m5.2xlarge"
	}
	return "t4g.2xlarge"
}

func (u Ubuntu2204) AMIName(ctx context.Context, awsSession *session.Session) (string, error) {
	amiId, err := getAmiIDFromSSM(ctx, ssm.New(awsSession), "/aws/service/canonical/ubuntu/server/22.04/stable/current/"+u.Architecture+"/hvm/ebs-gp2/ami-id")
	return *amiId, err
}

func (u Ubuntu2204) BuildUserData(userDataInput UserDataInput) ([]byte, error) {
	if err := populateBaseScripts(&userDataInput); err != nil {
		return nil, err
	}

	data := ubuntuCloudInitData{
		UserDataInput: userDataInput,
		NodeadmUrl:    userDataInput.NodeadmUrls.AMD,
	}

	if u.Architecture == arm64Arch {
		data.NodeadmUrl = userDataInput.NodeadmUrls.ARM
	}

	if u.ContainerdSource == "docker" {
		data.NodeadmAdditionalArgs = "--containerd-source docker"
	}

	return executeTemplate(ubuntu2204CloudInit, data)
}

type Ubuntu2404 struct {
	Architecture     string
	ContainerdSource string
}

func NewUbuntu2404AMD() *Ubuntu2404 {
	u := new(Ubuntu2404)
	u.Architecture = "amd64"
	u.ContainerdSource = "distro"
	return u
}

func NewUbuntu2404DockerSource() *Ubuntu2404 {
	u := new(Ubuntu2404)
	u.Architecture = "amd64"
	u.ContainerdSource = "docker"
	return u
}

func NewUbuntu2404ARM() *Ubuntu2404 {
	u := new(Ubuntu2404)
	u.Architecture = arm64Arch
	u.ContainerdSource = "distro"
	return u
}

func (u Ubuntu2404) Name() string {
	name := "ubuntu2404-" + u.Architecture
	if u.ContainerdSource == "docker" {
		name += "-docker"
	}
	return name
}

func (u Ubuntu2404) InstanceType() string {
	if u.Architecture == "amd64" {
		return "m5.2xlarge"
	}
	return "t4g.2xlarge"
}

func (u Ubuntu2404) AMIName(ctx context.Context, awsSession *session.Session) (string, error) {
	amiId, err := getAmiIDFromSSM(ctx, ssm.New(awsSession), "/aws/service/canonical/ubuntu/server/24.04/stable/current/"+u.Architecture+"/hvm/ebs-gp3/ami-id")
	return *amiId, err
}

func (u Ubuntu2404) BuildUserData(userDataInput UserDataInput) ([]byte, error) {
	if err := populateBaseScripts(&userDataInput); err != nil {
		return nil, err
	}

	data := ubuntuCloudInitData{
		UserDataInput: userDataInput,
		NodeadmUrl:    userDataInput.NodeadmUrls.AMD,
	}

	if u.Architecture == arm64Arch {
		data.NodeadmUrl = userDataInput.NodeadmUrls.ARM
	}

	if u.ContainerdSource == "docker" {
		data.NodeadmAdditionalArgs = "--containerd-source docker"
	}

	return executeTemplate(ubuntu2404CloudInit, data)
}

func populateBaseScripts(userDataInput *UserDataInput) error {
	logCollector, err := executeTemplate(logCollectorScript, userDataInput)
	if err != nil {
		return err
	}

	userDataInput.Files = append(userDataInput.Files,
		File{Content: string(nodeAdmInitScript), Path: "/tmp/nodeadm-init.sh", Permissions: "0755"},
		File{Content: string(logCollector), Path: "/tmp/log-collector.sh", Permissions: "0755"},
	)
	return nil
}

func executeTemplate(templateData []byte, values any) ([]byte, error) {
	tmpl, err := template.New("cloud-init").Funcs(templateFuncMap()).Parse(string(templateData))
	if err != nil {
		return nil, err
	}

	// Execute the template and write the result to a buffer
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
