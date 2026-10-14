package workflow

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("workflow not found")
	ErrCallNotFound  = errors.New("call not found")
	ErrWrongOwner    = errors.New("workflow belongs to another user")
	ErrInvalid       = errors.New("invalid workflow")
	ErrNameConflict  = errors.New("variable name conflict")
	ErrUnknownVar    = errors.New("unknown variable")
	ErrVarOutOfOrder = errors.New("variable produced by a later call")
)

// ValidationError names the call and the variable at fault, so the message can
// be shown as is: `call "detail": {{first_id}} is produced by "liste", placed after it`.
type ValidationError struct {
	Call     string
	Variable string
	Message  string
	Kind     error
}

func (e *ValidationError) Error() string {
	switch {
	case e.Call != "" && e.Variable != "":
		return fmt.Sprintf("call %q, variable {{%s}}: %s", e.Call, e.Variable, e.Message)
	case e.Call != "":
		return fmt.Sprintf("call %q: %s", e.Call, e.Message)
	case e.Variable != "":
		return fmt.Sprintf("variable {{%s}}: %s", e.Variable, e.Message)
	default:
		return e.Message
	}
}

func (e *ValidationError) Unwrap() []error {
	if e.Kind != nil {
		return []error{ErrInvalid, e.Kind}
	}
	return []error{ErrInvalid}
}
