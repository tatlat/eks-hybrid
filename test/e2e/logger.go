package e2e

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LoggerConfig struct {
	NoColor bool
}

func (c LoggerConfig) Apply(opts *LoggerConfig) {
	opts.NoColor = c.NoColor
}

type LoggerOption interface {
	Apply(*LoggerConfig)
}

func NewLogger(opts ...LoggerOption) logr.Logger {
	cfg := &LoggerConfig{}
	for _, opt := range opts {
		opt.Apply(cfg)
	}

	encoderCfg := zap.NewDevelopmentEncoderConfig()
	if !cfg.NoColor {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	consoleEncoder := zapcore.NewConsoleEncoder(encoderCfg)
	core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	log := zap.New(core)
	return zapr.NewLogger(log)
}
