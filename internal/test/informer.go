package test

import (
	"context"

	"github.com/aws/eks-hybrid/internal/validation"
)

// FakeInformer is a fake implementation of [validation.Informer].
type FakeInformer struct {
	// Started indicates if Starting was called.
	Started bool
	// DoneWith is the error passed to Done.
	DoneWith error
}

var _ validation.Informer = &FakeInformer{}

// NewFakeInformer returns a FakeInformer.
func NewFakeInformer() *FakeInformer {
	return &FakeInformer{}
}

func (f *FakeInformer) Starting(ctx context.Context, name, message string) {
	f.Started = true
}

func (f *FakeInformer) Done(ctx context.Context, name string, err error) {
	f.DoneWith = err
}
