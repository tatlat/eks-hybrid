package daemon

import (
	"context"
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
