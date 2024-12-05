package cni

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"

	"k8s.io/client-go/dynamic"
)

//go:embed testdata/calico/tigera-operator.yaml
var tigeraTemplate []byte

//go:embed testdata/calico/calico-template.yaml
var calicoTemplate []byte

type Calico struct {
	K8s dynamic.Interface
	// podCIDR is the cluster level CIDR to be use for Pods. It needs to be big enough for
	// Hybrid Nodes.
	//
	// Check the calico-template file for the node pod cidr mask. The default is 24.
	podCIDR string
}

func NewCalico(k8s dynamic.Interface, podCIDR string) Calico {
	return Calico{
		K8s:     k8s,
		podCIDR: podCIDR,
	}
}

// Deploy creates or updates the Calico reosurces.
func (c Calico) Deploy(ctx context.Context) error {
	tmpl, err := template.New("calico").Parse(string(calicoTemplate))
	if err != nil {
		return err
	}
	values := map[string]string{
		"PodCIDR": c.podCIDR,
	}
	installation := &bytes.Buffer{}
	err = tmpl.Execute(installation, values)
	if err != nil {
		return err
	}

	objs, err := yamlToUnstructured(append(tigeraTemplate, installation.Bytes()...))
	if err != nil {
		return err
	}

	fmt.Println("Applying calico installation")

	return upsertManifests(ctx, c.K8s, objs)
}
