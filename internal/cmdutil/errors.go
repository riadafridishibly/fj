package cmdutil

import (
	"errors"
	"fmt"
)

// SilentError is an error that should not be printed, only the exit code matters.
var SilentError = errors.New("SilentError")

// FlagErrorf creates a flag-related error. Cobra will show usage when this is returned.
func FlagErrorf(format string, args ...interface{}) error {
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

// AuthError indicates the user needs to authenticate.
type AuthError struct {
	err error
}

func NewAuthError(format string, args ...interface{}) *AuthError {
	return &AuthError{err: fmt.Errorf(format, args...)}
}

func (e *AuthError) Error() string {
	return e.err.Error()
}

func IsAuthError(err error) bool {
	var ae *AuthError
	return errors.As(err, &ae)
}
