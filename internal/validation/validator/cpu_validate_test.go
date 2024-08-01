package validator_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/validation/validator"
)

func TestCPUValidateFail(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNumCPU(FakeCPUFail)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.FailError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func TestCPUValidateWarning(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNumCPU(FakeCPUWarning)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.WarningError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func TestCPUValidatePass(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNumCPU(FakeCPUPass)
	r.Register(n)

	g.Expect(r.Run()).To(Succeed())
}

func FakeCPUFail() int {
	return 0
}

func FakeCPUWarning() int {
	return 1
}

func FakeCPUPass() int {
	return 2
}
