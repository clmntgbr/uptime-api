package expiry

import "errors"

var (
	ErrNotFound          = errors.New("expiry resource not found")
	ErrUnknownType       = errors.New("unknown expiry type")
	ErrInvalidThresholds = errors.New("invalid thresholds")
	ErrWrongOwner        = errors.New("workflow belongs to another user")
)
