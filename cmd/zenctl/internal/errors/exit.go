package errors

import "fmt"

// ExitError represents a command error with a specific exit code
type ExitError struct {
	Code int
	Err  error
}

// Error implements the error interface
func (e *ExitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("exit code %d: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

// Unwrap returns the underlying error
func (e *ExitError) Unwrap() error {
	return e.Err
}

// NewExitError creates a new ExitError with the given exit code and error
func NewExitError(code int, err error) *ExitError {
	return &ExitError{Code: code, Err: err}
}

