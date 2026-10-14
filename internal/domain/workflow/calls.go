package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	domaincall "go-api/internal/domain/call"

	"github.com/google/uuid"
)

// AddCall appends a call at the end of the workflow.
func (w *Workflow) AddCall(def domaincall.Definition, globalNames []string) (domaincall.Call, error) {
	if len(w.Calls) >= MaxCalls {
		return domaincall.Call{}, &ValidationError{Message: fmt.Sprintf("a workflow has at most %d calls", MaxCalls)}
	}
	for _, existing := range w.Calls {
		if existing.Name == strings.TrimSpace(def.Name) {
			return domaincall.Call{}, &ValidationError{Call: existing.Name, Message: "call names must be unique within a workflow"}
		}
	}

	call, err := domaincall.New(def, len(w.Calls)+1, uuid.New())
	if err != nil {
		return domaincall.Call{}, mapCallError(err)
	}

	next := append(append([]domaincall.Call{}, w.Calls...), call)
	if err := validateVariables(w.Statics, next, globalNames); err != nil {
		return domaincall.Call{}, err
	}
	w.Calls = next
	w.recordCallMutation()
	return call, nil
}

// ReplaceCall updates an existing call in place (same id and position).
func (w *Workflow) ReplaceCall(callID uuid.UUID, def domaincall.Definition, globalNames []string) (domaincall.Call, error) {
	index := -1
	for i, c := range w.Calls {
		if c.ID == callID {
			index = i
			break
		}
	}
	if index < 0 {
		return domaincall.Call{}, ErrCallNotFound
	}

	call, err := domaincall.New(def, w.Calls[index].Position, callID)
	if err != nil {
		return domaincall.Call{}, mapCallError(err)
	}
	for i, existing := range w.Calls {
		if i != index && existing.Name == call.Name {
			return domaincall.Call{}, &ValidationError{Call: call.Name, Message: "call names must be unique within a workflow"}
		}
	}

	next := append([]domaincall.Call{}, w.Calls...)
	next[index] = call
	if err := validateVariables(w.Statics, next, globalNames); err != nil {
		return domaincall.Call{}, err
	}
	w.Calls = next
	w.recordCallMutation()
	return call, nil
}

// RemoveCall deletes a call and renumbers the remaining positions.
func (w *Workflow) RemoveCall(callID uuid.UUID, globalNames []string) error {
	next := make([]domaincall.Call, 0, len(w.Calls))
	found := false
	for _, c := range w.Calls {
		if c.ID == callID {
			found = true
			continue
		}
		next = append(next, c)
	}
	if !found {
		return ErrCallNotFound
	}
	for i := range next {
		next[i].Position = i + 1
	}
	if err := validateVariables(w.Statics, next, globalNames); err != nil {
		return err
	}
	w.Calls = next
	w.recordCallMutation()
	return nil
}

func (w *Workflow) recordCallMutation() {
	w.UpdatedAt = time.Now().UTC()
	w.recordEvent(WorkflowUpdated{
		ID:              uuid.New().String(),
		WorkflowID:      w.ID.String(),
		UserID:          w.UserID.String(),
		Name:            w.Name,
		IntervalSeconds: int(w.Interval / time.Second),
		Enabled:         w.Enabled,
		CallCount:       len(w.Calls),
		Timestamp:       w.UpdatedAt,
	})
}

func mapCallError(err error) error {
	var cerr *domaincall.ValidationError
	if errors.As(err, &cerr) {
		return &ValidationError{Call: cerr.Call, Message: cerr.Message}
	}
	return err
}
