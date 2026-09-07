package apperror

import "fmt"

type Code string

const (
	CodeInvalidArgument Code = "invalid_argument"
	CodeConflict        Code = "conflict"
	CodeNotFound        Code = "not_found"
	CodeUnauthorized    Code = "unauthorized"
	CodeForbidden       Code = "forbidden"
	CodeProviderFailure Code = "provider_failure"
	CodeInternal        Code = "internal"
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }
