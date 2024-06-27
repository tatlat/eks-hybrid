package logger

import (
	"os"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Init() {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:    "message",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
		EncodeName:    zapcore.FullNameEncoder,
	}
	encoder := zapcore.NewConsoleEncoder(encoderCfg)
	stdoutSyncer := zapcore.Lock(os.Stdout)
	core := zapcore.NewCore(encoder, stdoutSyncer, zapcore.DebugLevel)
	logger := zap.New(core)
	SetLogger(zapr.NewLogger(logger))
}
