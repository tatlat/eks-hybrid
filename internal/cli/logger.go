package cli

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(opts *GlobalOptions) *zap.Logger {
	var logger *zap.Logger
	var err error

	if opts.DevelopmentMode {
		logger, err = zap.NewDevelopment()
	} else {
		config := zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.DisableStacktrace = true
		logger, err = config.Build()
	}
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
	return logger
}
