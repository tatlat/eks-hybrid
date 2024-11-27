package errors

// Silent is an error that should not be printed.
// Useful when errors and printed/presented during command
// execution and it doesn't need to be printed again after the command
// returns.
type Silent struct {
	error
}

// NewSilent returns a new Silent.
func NewSilent(err error) error {
	return Silent{
		error: err,
	}
}

// IsSilent checks if an error is Silent.
func IsSilent(err error) bool {
	_, ok := err.(Silent)
	return ok
}
