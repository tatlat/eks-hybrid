package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// WaitForStatus waits for the daemon to reach the desired status.
// It will keep checking the status of the daemon until it reaches the desired status
// or the context is cancelled. If you don't cancel the context, this function will
// keep retrying indefinitely.
func WaitForStatus(ctx context.Context, logger *zap.Logger, manager DaemonManager, daemonName string, desired DaemonStatus, backoff time.Duration) error {
	for {
		status, err := manager.GetDaemonStatus(daemonName)
		if err != nil {
			logger.Error("Failed to get daemon status", zap.String("daemon", daemonName), zap.Error(err))
		} else {
			if status == desired {
				return nil
			}
			logger.Info("Daemon is not in the desired state yet", zap.String("daemon", daemonName), zap.String("status", string(status)))
		}
		logger.Debug("Waiting before next retry", zap.Duration("backoff", backoff))
		select {
		case <-ctx.Done():
			return fmt.Errorf("daemon %s still has status %s: %w", daemonName, status, ctx.Err())
		case <-time.After(backoff):
		}
	}
}

// AsyncOperation is a daemon operation that performs an action in the background.
type AsyncOperation func(ctx context.Context, daemonName string, opts ...OperationOption) error

// WaitForOperation waits for an asynchronous operation to complete. If the operation returns a Failed or Timeout
// result, an error is returned. If the context is cancelled, an error is returned. If the operation completes
// with any other result (Done, Cancelled, Dependency, Skipped), nil is returned.
func WaitForOperation(ctx context.Context, op AsyncOperation, name string, opts ...OperationOption) error {
	o := &OperationOptions{}
	for _, opt := range opts {
		opt(o)
	}

	if o.Result != nil {
		return errors.New("cannot specify a result channel when waiting for an operation")
	}

	result := make(chan OperationResult)
	o.Result = result

	if err := op(ctx, name, o.ApplyAll); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("operation for daemon %s did not complete in time, result is unknown: %w", name, ctx.Err())
	case res := <-result:
		switch res {
		case Failed, Timeout:
			return fmt.Errorf("operation for daemon %s failed with result [%s]", name, res)
		case Done, Canceled, Dependency, Skipped:
			return nil
		}
	}

	return nil
}
