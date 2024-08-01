package validator_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/validation/validator"
)

func TestSysMemValidateFail(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewSysMem(FakeSysMemFail)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.FailError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func TestSysMemValidateWarning(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewSysMem(FakeSysMemWarning)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.WarningError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func TestSysMemValidatePass(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewSysMem(FakeSysMemPass)
	r.Register(n)

	g.Expect(r.Run()).To(Succeed())
}

func FakeSysMemFail() uint64 {
	return uint64(1) * validator.GB
}

func FakeSysMemWarning() uint64 {
	return uint64(3) * validator.GB
}

func FakeSysMemPass() uint64 {
	return uint64(5) * validator.GB
}
