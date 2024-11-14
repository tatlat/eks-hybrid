package e2e

import (
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() logr.Logger {
	encoderCfg := zap.NewDevelopmentEncoderConfig()
	encoderCfg.EncodeLevel = nil
	encoderCfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {}

	consoleEncoder := zapcore.NewConsoleEncoder(encoderCfg)
	core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	log := zap.New(core)
	return zapr.NewLogger(log)
}
