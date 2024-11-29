package logger

import (
	"context"

	"go.uber.org/zap"
)

type contextKey struct{}

var key = contextKey{}

// FromContext returns a Logger from the context.
// If no logger is found, a no-op logger is returned.
func FromContext(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(key).(*zap.Logger)
	if !ok {
		return zap.NewNop()
	}
	return logger
}

// NewContext returns a new Context, derived from ctx, which carries the
// provided Logger.
func NewContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, key, logger)
}
