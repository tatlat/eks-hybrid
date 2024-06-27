package validator

import (
	"fmt"
)

type Validator interface {
	Validate() error
}

type Runner struct {
	validations []Validator
}

func NewRunner() *Runner {
	return &Runner{validations: make([]Validator, 0)}
}

func (r *Runner) Register(validations ...Validator) {
	r.validations = append(r.validations, validations...)
}

func (r *Runner) Run() error {
	var errs []error
	for _, v := range r.validations {
		err := v.Validate()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(FormatErrors(errs))
	}
	return nil
}
