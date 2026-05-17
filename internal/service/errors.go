package service

import "errors"

var (
	ErrNotFound            = errors.New("notification task not found")
	ErrIdempotencyConflict = errors.New("idempotency key already used with different request content")
	ErrInvalidNotification = errors.New("invalid notification request")
	ErrForbidden           = errors.New("app is not allowed to use vendor")
)
