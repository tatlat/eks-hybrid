package logger_test

import (
	"testing"

	"github.com/aws/eks-hybrid/internal/validation/logger"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
)

func TestLogInitSuccess(t *testing.T) {
	reset()
	g := NewWithT(t)
	logger.Init()
	l := logger.Get()
	g.Expect(l.GetSink()).To(Not(BeNil()))
}

func TestLogInitFail(t *testing.T) {
	reset()
	g := NewWithT(t)
	l := logger.Get()
	g.Expect(l.GetSink()).To(BeNil())
}

func reset() {
	logger.SetLogger(logr.Discard())
}
