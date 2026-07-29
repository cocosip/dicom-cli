// Package apperr defines application error categories and process exit codes.
package apperr

import "errors"

// Kind identifies the category used to select a process exit code.
type Kind uint8

const (
	KindOperation Kind = iota
	KindInput
	KindValidation
)

// Error adds an application error category while preserving the root cause.
type Error struct {
	Kind Kind
	Err  error
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Wrap classifies err. A nil error remains nil.
func Wrap(kind Kind, err error) error {
	if err == nil {
		return nil
	}

	return &Error{Kind: kind, Err: err}
}

// ExitCode maps application errors to the CLI exit-code contract.
func ExitCode(err error) int {
	var appErr *Error
	if errors.As(err, &appErr) {
		switch appErr.Kind {
		case KindInput:
			return 2
		case KindValidation:
			return 3
		}
	}

	return 1
}
