package workflow

import (
	"fmt"
	"strings"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

// AddStatic declares a workflow-scoped variable (no extraction path).
func (w *Workflow) AddStatic(name, value string, globalNames []string) (StaticVariable, error) {
	name = strings.TrimSpace(name)
	if err := variable.ValidateName(name); err != nil {
		return StaticVariable{}, &ValidationError{Variable: name, Message: err.Error()}
	}
	if err := variable.ValidateValue(name, value); err != nil {
		return StaticVariable{}, &ValidationError{Variable: name, Message: err.Error()}
	}
	for _, s := range w.Statics {
		if s.Name == name {
			return StaticVariable{}, variable.ErrAlreadyExists
		}
	}
	next := append(append([]StaticVariable{}, w.Statics...), StaticVariable{
		ID: uuid.New(), Name: name, Value: value,
	})
	if err := validateVariables(next, w.Calls, globalNames); err != nil {
		return StaticVariable{}, err
	}
	w.Statics = next
	w.recordCallMutation()
	return w.Statics[len(w.Statics)-1], nil
}

// UpdateStatic replaces the value of an existing static variable.
func (w *Workflow) UpdateStatic(name, value string, globalNames []string) (StaticVariable, error) {
	name = strings.TrimSpace(name)
	index := -1
	for i, s := range w.Statics {
		if s.Name == name {
			index = i
			break
		}
	}
	if index < 0 {
		return StaticVariable{}, ErrVariableNotFound
	}
	if err := variable.ValidateValue(name, value); err != nil {
		return StaticVariable{}, &ValidationError{Variable: name, Message: err.Error()}
	}
	next := append([]StaticVariable{}, w.Statics...)
	next[index].Value = value
	if err := validateVariables(next, w.Calls, globalNames); err != nil {
		return StaticVariable{}, err
	}
	w.Statics = next
	w.recordCallMutation()
	return w.Statics[index], nil
}

// RemoveStatic deletes a static variable when no call references it.
func (w *Workflow) RemoveStatic(name string, globalNames []string) error {
	name = strings.TrimSpace(name)
	if w.UsesVariable(name) {
		return fmt.Errorf("%w: %q", ErrVariableInUse, name)
	}
	next := make([]StaticVariable, 0, len(w.Statics))
	found := false
	for _, s := range w.Statics {
		if s.Name == name {
			found = true
			continue
		}
		next = append(next, s)
	}
	if !found {
		return ErrVariableNotFound
	}
	if err := validateVariables(next, w.Calls, globalNames); err != nil {
		return err
	}
	w.Statics = next
	w.recordCallMutation()
	return nil
}

// FindStatic returns a static by name, or nil when missing.
func (w *Workflow) FindStatic(name string) *StaticVariable {
	name = strings.TrimSpace(name)
	for i := range w.Statics {
		if w.Statics[i].Name == name {
			s := w.Statics[i]
			return &s
		}
	}
	return nil
}

// AddCallVariable declares an extraction on a call (optional path / expression).
func (w *Workflow) AddCallVariable(callID uuid.UUID, extraction domaincall.Extraction, globalNames []string) (domaincall.Extraction, error) {
	index, err := w.callIndex(callID)
	if err != nil {
		return domaincall.Extraction{}, err
	}
	normalized, err := domaincall.NormalizeExtraction(extraction)
	if err != nil {
		return domaincall.Extraction{}, &ValidationError{Call: w.Calls[index].Name, Variable: extraction.Name, Message: err.Error()}
	}
	for _, e := range w.Calls[index].Extractions {
		if e.Name == normalized.Name {
			return domaincall.Extraction{}, variable.ErrAlreadyExists
		}
	}
	nextCalls := append([]domaincall.Call{}, w.Calls...)
	nextCalls[index].Extractions = append(append([]domaincall.Extraction{}, nextCalls[index].Extractions...), normalized)
	if err := validateVariables(w.Statics, nextCalls, globalNames); err != nil {
		return domaincall.Extraction{}, err
	}
	w.Calls = nextCalls
	w.recordCallMutation()
	return normalized, nil
}

// UpdateCallVariable replaces an extraction on a call (matched by current name).
func (w *Workflow) UpdateCallVariable(callID uuid.UUID, name string, extraction domaincall.Extraction, globalNames []string) (domaincall.Extraction, error) {
	index, err := w.callIndex(callID)
	if err != nil {
		return domaincall.Extraction{}, err
	}
	name = strings.TrimSpace(name)
	extIndex := -1
	for i, e := range w.Calls[index].Extractions {
		if e.Name == name {
			extIndex = i
			break
		}
	}
	if extIndex < 0 {
		return domaincall.Extraction{}, ErrVariableNotFound
	}
	normalized, err := domaincall.NormalizeExtraction(extraction)
	if err != nil {
		return domaincall.Extraction{}, &ValidationError{Call: w.Calls[index].Name, Variable: name, Message: err.Error()}
	}
	if normalized.Name != name {
		for _, e := range w.Calls[index].Extractions {
			if e.Name == normalized.Name {
				return domaincall.Extraction{}, variable.ErrAlreadyExists
			}
		}
	}
	nextCalls := append([]domaincall.Call{}, w.Calls...)
	nextExt := append([]domaincall.Extraction{}, nextCalls[index].Extractions...)
	nextExt[extIndex] = normalized
	nextCalls[index].Extractions = nextExt
	if err := validateVariables(w.Statics, nextCalls, globalNames); err != nil {
		return domaincall.Extraction{}, err
	}
	w.Calls = nextCalls
	w.recordCallMutation()
	return normalized, nil
}

// RemoveCallVariable deletes an extraction when later calls do not reference it.
func (w *Workflow) RemoveCallVariable(callID uuid.UUID, name string, globalNames []string) error {
	index, err := w.callIndex(callID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if w.UsesVariable(name) {
		return fmt.Errorf("%w: %q", ErrVariableInUse, name)
	}
	nextExt := make([]domaincall.Extraction, 0, len(w.Calls[index].Extractions))
	found := false
	for _, e := range w.Calls[index].Extractions {
		if e.Name == name {
			found = true
			continue
		}
		nextExt = append(nextExt, e)
	}
	if !found {
		return ErrVariableNotFound
	}
	nextCalls := append([]domaincall.Call{}, w.Calls...)
	nextCalls[index].Extractions = nextExt
	if err := validateVariables(w.Statics, nextCalls, globalNames); err != nil {
		return err
	}
	w.Calls = nextCalls
	w.recordCallMutation()
	return nil
}

// FindCallVariable returns an extraction by name on a call.
func (w *Workflow) FindCallVariable(callID uuid.UUID, name string) (*domaincall.Extraction, error) {
	index, err := w.callIndex(callID)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	for _, e := range w.Calls[index].Extractions {
		if e.Name == name {
			cp := e
			return &cp, nil
		}
	}
	return nil, ErrVariableNotFound
}

// ListCallVariables returns extractions declared on a call.
func (w *Workflow) ListCallVariables(callID uuid.UUID) ([]domaincall.Extraction, error) {
	index, err := w.callIndex(callID)
	if err != nil {
		return nil, err
	}
	out := make([]domaincall.Extraction, len(w.Calls[index].Extractions))
	copy(out, w.Calls[index].Extractions)
	return out, nil
}

// AvailableVariables lists globals, workflow statics and extractions from calls
// that run before callID (what that call may reference).
func (w *Workflow) AvailableVariables(callID uuid.UUID, globals []variable.Global) ([]variable.Available, error) {
	index, err := w.callIndex(callID)
	if err != nil {
		return nil, err
	}
	out := make([]variable.Available, 0, len(globals)+len(w.Statics)+8)
	for _, g := range globals {
		out = append(out, variable.Available{
			Name: g.Name, Type: variable.TypeGlobal, Secret: g.Secret, Value: g.Value,
		})
	}
	for _, s := range w.Statics {
		out = append(out, variable.Available{
			Name: s.Name, Type: variable.TypeStatic, Value: s.Value,
		})
	}
	for i := 0; i < index; i++ {
		c := w.Calls[i]
		for _, e := range c.Extractions {
			out = append(out, variable.Available{
				Name:     e.Name,
				Type:     variable.TypeCall,
				Path:     e.Expression,
				Source:   string(e.Source),
				CallID:   c.ID,
				CallName: c.Name,
			})
		}
	}
	return out, nil
}

func (w *Workflow) callIndex(callID uuid.UUID) (int, error) {
	for i, c := range w.Calls {
		if c.ID == callID {
			return i, nil
		}
	}
	return -1, ErrCallNotFound
}
