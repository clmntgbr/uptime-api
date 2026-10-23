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
// Call variables are owned by AddCallVariable / UpdateCallVariable / RemoveCallVariable
// and are never replaced from the call definition.
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

	def.Extractions = append([]domaincall.Extraction{}, w.Calls[index].Extractions...)
	call, err := domaincall.New(def, w.Calls[index].Position, callID)
	if err != nil {
		return domaincall.Call{}, mapCallError(err)
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
	w.recordEvent(w.newUpdatedOrCreatedEvent(false, w.UpdatedAt))
}

func mapCallError(err error) error {
	var cerr *domaincall.ValidationError
	if errors.As(err, &cerr) {
		return &ValidationError{Call: cerr.Call, Message: cerr.Message}
	}
	return err
}

// withPreservedExtractionIDs copies existing extraction IDs matched by name
// onto the definition so ReplaceCall / apply do not rotate stable variable IDs.
func withPreservedExtractionIDs(def domaincall.Definition, existing []domaincall.Extraction) domaincall.Definition {
	if len(def.Extractions) == 0 || len(existing) == 0 {
		return def
	}
	byName := make(map[string]uuid.UUID, len(existing))
	for _, e := range existing {
		byName[e.Name] = e.ID
	}
	out := make([]domaincall.Extraction, len(def.Extractions))
	copy(out, def.Extractions)
	for i := range out {
		if out[i].ID != uuid.Nil {
			continue
		}
		if id, ok := byName[strings.TrimSpace(out[i].Name)]; ok && id != uuid.Nil {
			out[i].ID = id
		}
	}
	def.Extractions = out
	return def
}
