package validator_test

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/validation/validator"
)

func TestDiskValidateFail(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewDiskSize(FakeDiskFail)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.FailError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func TestDiskValidateWarning(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewDiskSize(FakeDiskWarning)
	r.Register(n)

	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.WarningError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func TestDiskValidatePass(t *testing.T) {
	g := NewWithT(t)
	r := validator.NewRunner()
	n := validator.NewDiskSize(FakeDiskPass)
	r.Register(n)

	g.Expect(r.Run()).To(Succeed())
}

func FakeDiskFail() (float64, error) {
	err := errors.New("Error")
	return 0, err
}

func FakeDiskWarning() (float64, error) {
	return float64(10), nil
}

func FakeDiskPass() (float64, error) {
	return float64(20), nil
}
