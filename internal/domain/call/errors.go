package call

import (
	"errors"
	"fmt"
)

var ErrInvalid = errors.New("invalid call")

// ValidationError names the call at fault, so the message can be shown as is.
type ValidationError struct {
	Call    string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Call == "" {
		return e.Message
	}
	return fmt.Sprintf("call %q: %s", e.Call, e.Message)
}

func (e *ValidationError) Unwrap() error { return ErrInvalid }
