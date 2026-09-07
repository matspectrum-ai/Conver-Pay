package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	ErrNoEligibleProvider  = errors.New("no eligible provider")
	ErrInvalidTransition   = errors.New("invalid state transition")
)
