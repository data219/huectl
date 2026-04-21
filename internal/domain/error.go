package domain

import "fmt"

const (
	ExitSuccess  = 0
	ExitUsage    = 1
	ExitPartial  = 2
	ExitAuth     = 3
	ExitTarget   = 4
	ExitNetwork  = 5
	ExitRetry    = 6
	ExitInternal = 7
)

type AppError struct {
	Code       string
	Message    string
	Hints      []string
	Details    map[string]any
	ExitCode   int
	Temporary  bool
	Underlying error
}

func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Underlying == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Underlying)
}

func WrapError(code string, message string, exitCode int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		ExitCode:   exitCode,
		Underlying: err,
	}
}

func NewError(code string, message string, exitCode int) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		ExitCode: exitCode,
	}
}
