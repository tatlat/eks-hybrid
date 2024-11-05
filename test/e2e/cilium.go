package e2e

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

//go:embed testdata/cilium-template.yaml
var ciliumTemplate []byte

type cilium struct {
	K8s dynamic.Interface
	// PodCIDR is the cluster level CIDR to be use for Pods. It needs to be big enough for
	// Hybrid Nodes.
	//
	// Check the cilium-template file for the node pod cidr mask. The default is 24.
	PodCIDR string
}

func newCilium(k8s dynamic.Interface, podCIDR string) cilium {
	return cilium{
		K8s:     k8s,
		PodCIDR: podCIDR,
	}
}

// deploy creates or updates the Cilium reosurces.
func (c cilium) deploy(ctx context.Context) error {
	tmpl, err := template.New("cilium").Parse(string(ciliumTemplate))
	if err != nil {
		return err
	}
	values := make(map[string]string)
	values["PodCIDR"] = c.PodCIDR
	installation := &bytes.Buffer{}
	err = tmpl.Execute(installation, values)
	if err != nil {
		return err
	}

	objs, err := yamlToUnstructured(installation.Bytes())
	if err != nil {
		return err
	}

	fmt.Println("Applying cilium installation")
	for _, obj := range objs {
		o := obj
		groupVersion := o.GroupVersionKind()
		resource := schema.GroupVersionResource{
			Group:    groupVersion.Group,
			Version:  groupVersion.Version,
			Resource: strings.ToLower(groupVersion.Kind + "s"),
		}
		k8s := c.K8s.Resource(resource).Namespace(obj.GetNamespace())
		if _, err := k8s.Get(ctx, obj.GetName(), metav1.GetOptions{}); apierrors.IsNotFound(err) {
			fmt.Printf("Creating cilium object %s (%s)", o.GetName(), groupVersion)
			_, err := k8s.Create(ctx, &obj, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating cilium object %s (%s): %w", o.GetName(), groupVersion, err)
			}
		} else if err != nil {
			return fmt.Errorf("reading cilium object %s (%s): %w", o.GetName(), groupVersion, err)
		} else {
			fmt.Printf("Updating cilium object %s (%s)", o.GetName(), groupVersion)
			_, err := k8s.Update(ctx, &obj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("updating cilium object %s (%s): %w", o.GetName(), groupVersion, err)
			}
		}
	}

	return nil
}
