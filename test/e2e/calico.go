package e2e

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

type calico struct {
	K8s dynamic.Interface
	// PodCIDR is the cluster level CIDR to be use for Pods. It needs to be big enough for
	// Hybrid Nodes.
	//
	// Check the calico-template file for the node pod cidr mask. The default is 24.
	PodCIDR string
}

func newCalico(k8s dynamic.Interface, podCIDR string) calico {
	return calico{
		K8s:     k8s,
		PodCIDR: podCIDR,
	}
}

// deploy creates or updates the Calico reosurces.
func (c calico) deploy(ctx context.Context) error {
	tmpl, err := template.New("calico").Parse(string(calicoTemplate))
	if err != nil {
		return err
	}
	values := map[string]string{
		"PodCIDR": c.PodCIDR,
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
