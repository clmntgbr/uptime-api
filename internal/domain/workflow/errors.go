package workflow

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound         = errors.New("workflow not found")
	ErrCallNotFound     = errors.New("call not found")
	ErrVariableNotFound = errors.New("variable not found")
	ErrWrongOwner       = errors.New("workflow belongs to another user")
	ErrInvalid          = errors.New("invalid workflow")
	ErrNameConflict     = errors.New("variable name conflict")
	ErrUnknownVar       = errors.New("unknown variable")
	ErrVarOutOfOrder    = errors.New("variable produced by a later call")
	ErrRunInProgress    = errors.New("a run is already in progress")
	ErrNoRunInProgress  = errors.New("no run is in progress")
	ErrNoCalls          = errors.New("workflow has no calls")
	ErrVariableInUse    = errors.New("variable is used by a call")
	// ErrInvalidCallOrder is returned when reorder callIds is not an exact
	// permutation of the workflow calls (missing, extra, or duplicate).
	ErrInvalidCallOrder = errors.New("callIds must list every call exactly once")
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
