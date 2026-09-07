package provider

import "errors"

var (
	ErrInvalidCredentials              = errors.New("invalid provider credentials")
	ErrCredentialValidationUnavailable = errors.New("provider credential validation unavailable")
)
