package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// GetterDynamic retrieves an object of type O from the Kubernetes API.
// It matches the Get signature of client-go dynamic client.
type GetterDynamic[O runtime.Object] interface {
	Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (O, error)
}

// GetterFromDynamic converts a dynamic client to a Getter.
type GetterFromDynamic[O runtime.Object] struct {
	GetterDynamic[O]
}

func (g *GetterFromDynamic[O]) Get(ctx context.Context, name string, options metav1.GetOptions) (O, error) {
	return g.GetterDynamic.Get(ctx, name, options)
}

// GetterForDynamic makes a Getter from a dynamic client.
func GetterForDynamic[O runtime.Object](dyn GetterDynamic[O]) *GetterFromDynamic[O] {
	return &GetterFromDynamic[O]{
		GetterDynamic: dyn,
	}
}
