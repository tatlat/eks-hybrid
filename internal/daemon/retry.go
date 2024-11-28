package daemon

import (
	"context"
	"fmt"
	"time"
)

// RetryOperation retries an asynchronous operation until it succeeds or the context is cancelled.
// The backoff duration is the time to wait between retries.
// Each retry will wait for the operation to complete before retrying.
func RetryOperation(ctx context.Context, op AsyncOperation, name string, backoff time.Duration, opts ...OperationOption) error {
	retries := 0
	var err error
	for {
		err = WaitForOperation(ctx, op, name, opts...)
		if err == nil {
			return nil
		}
		retries++
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation didn't succeed after %d retries: %w", retries, err)
		case <-time.After(backoff):
		}
	}
}
