package kubernetes

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/aws/eks-hybrid/internal/retry"
)

// Getter retrieves an object of type O from the Kubernetes API.
// It matches the Get signature of client-go clients.
type Getter[O runtime.Object] interface {
	Get(ctx context.Context, name string, options metav1.GetOptions) (O, error)
}

// GetOptions configures a Get request.
type GetOptions struct {
	metav1.GetOptions
}

// GetOption is an option for the Get request.
type GetOption func(*GetOptions)

// GetRetry retries the get request until it succeeds or the retry limit is reached.
func GetRetry[O runtime.Object](ctx context.Context, getter Getter[O], name string, opts ...GetOption) (O, error) {
	getOpt := &GetOptions{}
	for _, opt := range opts {
		opt(getOpt)
	}

	var obj O
	err := retryRequest(ctx, func(ctx context.Context) error {
		var err error
		obj, err = getter.Get(ctx, name, getOpt.GetOptions)
		return err
	})

	return obj, err
}

// List retrieves a list of objects from the Kubernetes API.
// It matches the List signature of client-go clients.
type Lister[O runtime.Object] interface {
	List(context.Context, metav1.ListOptions) (O, error)
}

// ListOptions configures a List request.
type ListOptions struct {
	metav1.ListOptions
}

// ListOption is an option for the List request.
type ListOption func(*ListOptions)

// ListRetry retries the list request until it succeeds or the retry limit is reached.
func ListRetry[O runtime.Object](ctx context.Context, lister Lister[O], opts ...ListOption) (O, error) {
	listOpt := &ListOptions{}
	for _, opt := range opts {
		opt(listOpt)
	}

	var obj O
	err := retryRequest(ctx, func(ctx context.Context) error {
		var err error
		obj, err = lister.List(ctx, listOpt.ListOptions)
		return err
	})

	return obj, err
}

func retryRequest(ctx context.Context, request func(context.Context) error) error {
	return defaultRetrier().Do(ctx, func(ctx context.Context) (bool, error) {
		if err := request(ctx); err != nil {
			return false, err
		}
		return true, nil
	})
}

func defaultRetrier() *retry.Retrier {
	return &retry.Retrier{
		// The slowest operations to return are usually the requests that are throttled
		// by the API server, which clitn-go retries by itself. We have observed that
		// most of them return in 30s or less. If something takes longer, we just assume
		// it won't return, abort and retry.
		OperationTimeout: 30 * time.Second,
		// Regardless of the operation speed and the number of retries, we set the upper
		// bound time to 2 minutes, if we can't get a successful response in that time,
		// we assume this is not transient.
		Timeout: 2 * time.Minute,
		Backoff: retry.Backoff{
			// Classic exponential backoff with 10% jitter
			// to avoid syncronized requests.
			Duration: 1 * time.Second,
			Factor:   2,
			Jitter:   0.1,
			// We limit the number of retries in case the operations return fast,
			// to avoid retrying for up to the max timeout.
			Steps: 4,
		},
	}
}
