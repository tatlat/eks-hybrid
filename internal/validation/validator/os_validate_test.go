package validator_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/validation/validator"
)

func TestOSValidateUbuntuFail(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNodeOS(FakeOSUbuntuFail)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.FailError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}
func TestOSValidateUbuntuPass(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNodeOS(FakeOSUbuntuPass)
	r.Register(n)

	g.Expect(r.Run()).To(Succeed())
}

func TestOSValidateRhelFail(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNodeOS(FakeOSRhelFail)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.FailError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}
func TestOSValidateRhelPass(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNodeOS(FakeOSRhelPass)
	r.Register(n)

	g.Expect(r.Run()).To(Succeed())
}

func TestOSValidateGetOSError(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewNodeOS(FakeGetOSError)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.FailError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func FakeOSUbuntuFail() (validator.OS, error) {

	return validator.OS{"ubuntu", "18.04"}, nil
}

func FakeOSUbuntuPass() (validator.OS, error) {
	return validator.OS{"ubuntu", "20.04"}, nil
}

func FakeOSRhelFail() (validator.OS, error) {

	return validator.OS{"rhel", "7.6"}, nil
}

func FakeOSRhelPass() (validator.OS, error) {
	return validator.OS{"rhel", "9.4"}, nil
}

func FakeGetOSError() (validator.OS, error) {
	return validator.OS{}, fmt.Errorf("fake error")
}
