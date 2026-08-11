package cmdutil

import (
	"errors"
	"fmt"
)

// ErrSilent is an error that should not be printed, only the exit code matters.
var ErrSilent = errors.New("SilentError")

// FlagErrorf creates a flag-related error. Cobra will show usage when this is returned.
func FlagErrorf(format string, args ...any) error {
	return &FlagError{err: fmt.Errorf(format, args...)}
}

type FlagError struct {
	err error
}

func (e *FlagError) Error() string {
	return e.err.Error()
}

func (e *FlagError) Unwrap() error {
	return e.err
}

func IsFlagError(err error) bool {
	var fe *FlagError
	return errors.As(err, &fe)
}
