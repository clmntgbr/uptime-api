package incident

import "errors"

var (
	ErrNotFound       = errors.New("incident resource not found")
	ErrUnknownKind    = errors.New("unknown incident kind")
	ErrInvalidPolicy  = errors.New("invalid incident policy")
	ErrWrongOwner     = errors.New("workflow belongs to another user")
	ErrInvalidSeverity = errors.New("invalid severity")
)
