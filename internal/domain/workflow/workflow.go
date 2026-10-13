package workflow

import (
	"fmt"
	"strings"
	"time"

	"go-api/internal/domain/event"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

const (
	MaxCalls        = 20
	MaxVariables    = 50
	MaxNameLength   = 200
	MinInterval     = time.Minute
	WorkflowTimeout = 5 * time.Minute
)

type StaticVariable struct {
	ID    uuid.UUID
	Name  string
	Value string
}

// Definition is the user input to create or replace a workflow.
type Definition struct {
	Name     string
	Interval time.Duration
	Enabled  bool
	Statics  []StaticVariable
	Calls    []CallDefinition
}

// Workflow is an ordered list of calls run at a regular interval. Calls only
// see the variables produced before them: no branches, no conditions.
type Workflow struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Interval  time.Duration
	Timeout   time.Duration
	Enabled   bool
	Statics   []StaticVariable
	Calls     []Call
	CreatedAt time.Time
	UpdatedAt time.Time

	events []event.DomainEvent
}

// New validates the definition against the user's global variable names.
func New(userID uuid.UUID, def Definition, globalNames []string) (*Workflow, error) {
	now := time.Now().UTC()
	w := &Workflow{
		ID:        uuid.New(),
		UserID:    userID,
		Timeout:   WorkflowTimeout,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := w.apply(def, globalNames); err != nil {
		return nil, err
	}
	w.recordEvent(WorkflowCreated{
		ID:              uuid.New().String(),
		WorkflowID:      w.ID.String(),
		UserID:          userID.String(),
		Name:            w.Name,
		IntervalSeconds: int(w.Interval / time.Second),
		Enabled:         w.Enabled,
		CallCount:       len(w.Calls),
		Timestamp:       now,
	})
	return w, nil
}

// Update replaces the definition. Calls keep their ID when their name is kept,
// so results stay attached to the same call across edits. It reports whether
// the schedule changed (interval or enablement).
func (w *Workflow) Update(def Definition, globalNames []string) (bool, error) {
	previousInterval, previousEnabled := w.Interval, w.Enabled
	if err := w.apply(def, globalNames); err != nil {
		return false, err
	}
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
	return previousInterval != w.Interval || previousEnabled != w.Enabled, nil
}

func (w *Workflow) MarkDeleted() {
	w.recordEvent(WorkflowDeleted{
		ID:         uuid.New().String(),
		WorkflowID: w.ID.String(),
		UserID:     w.UserID.String(),
		Timestamp:  time.Now().UTC(),
	})
}

func (w *Workflow) PullEvents() []event.DomainEvent {
	events := w.events
	w.events = nil
	return events
}

func (w *Workflow) recordEvent(e event.DomainEvent) {
	w.events = append(w.events, e)
}

func (w *Workflow) apply(def Definition, globalNames []string) error {
	name := strings.TrimSpace(def.Name)
	if name == "" || len(name) > MaxNameLength {
		return &ValidationError{Message: fmt.Sprintf("workflow name is required (max %d characters)", MaxNameLength)}
	}
	if def.Interval < MinInterval || def.Interval%time.Minute != 0 {
		return &ValidationError{Message: "interval must be a whole number of minutes, at least 1 minute"}
	}
	if len(def.Calls) == 0 || len(def.Calls) > MaxCalls {
		return &ValidationError{Message: fmt.Sprintf("a workflow has between 1 and %d calls", MaxCalls)}
	}

	statics, err := buildStatics(w.Statics, def.Statics)
	if err != nil {
		return err
	}

	existingIDs := make(map[string]uuid.UUID, len(w.Calls))
	for _, c := range w.Calls {
		existingIDs[c.Name] = c.ID
	}

	calls := make([]Call, 0, len(def.Calls))
	callNames := map[string]struct{}{}
	for i, cd := range def.Calls {
		id, ok := existingIDs[strings.TrimSpace(cd.Name)]
		if !ok {
			id = uuid.New()
		}
		call, err := buildCall(cd, i+1, id)
		if err != nil {
			return err
		}
		if _, dup := callNames[call.Name]; dup {
			return &ValidationError{Call: call.Name, Message: "call names must be unique within a workflow"}
		}
		callNames[call.Name] = struct{}{}
		calls = append(calls, call)
	}

	if err := validateVariables(statics, calls, globalNames); err != nil {
		return err
	}

	w.Name = name
	w.Interval = def.Interval
	w.Enabled = def.Enabled
	w.Statics = statics
	w.Calls = calls
	return nil
}

func buildStatics(existing, defs []StaticVariable) ([]StaticVariable, error) {
	existingIDs := make(map[string]uuid.UUID, len(existing))
	for _, s := range existing {
		existingIDs[s.Name] = s.ID
	}
	out := make([]StaticVariable, 0, len(defs))
	for _, s := range defs {
		s.Name = strings.TrimSpace(s.Name)
		if err := variable.ValidateName(s.Name); err != nil {
			return nil, &ValidationError{Variable: s.Name, Message: err.Error()}
		}
		if err := variable.ValidateValue(s.Name, s.Value); err != nil {
			return nil, &ValidationError{Variable: s.Name, Message: err.Error()}
		}
		if id, ok := existingIDs[s.Name]; ok {
			s.ID = id
		} else if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		out = append(out, s)
	}
	return out, nil
}

// validateVariables enforces the resolution rules: one namespace across the
// three types, and a call only uses variables available before it runs.
func validateVariables(statics []StaticVariable, calls []Call, globalNames []string) error {
	origin := map[string]string{}
	for _, g := range globalNames {
		origin[g] = string(variable.TypeGlobal)
	}

	declare := func(name, from, callName string) error {
		if previous, ok := origin[name]; ok {
			return &ValidationError{
				Call:     callName,
				Variable: name,
				Message:  fmt.Sprintf("name already used by %s", previous),
				Kind:     ErrNameConflict,
			}
		}
		origin[name] = from
		return nil
	}

	for _, s := range statics {
		if err := declare(s.Name, "a static variable", ""); err != nil {
			return err
		}
	}

	producer := map[string]string{}
	for _, c := range calls {
		for _, e := range c.Extractions {
			if err := declare(e.Name, fmt.Sprintf("call %q", c.Name), c.Name); err != nil {
				return err
			}
			producer[e.Name] = c.Name
		}
	}

	if count := len(statics) + len(producer); count > MaxVariables {
		return &ValidationError{Message: fmt.Sprintf("a workflow declares at most %d variables (got %d)", MaxVariables, count)}
	}

	available := map[string]struct{}{}
	for _, name := range variable.Builtins() {
		available[name] = struct{}{}
	}
	for _, g := range globalNames {
		available[g] = struct{}{}
	}
	for _, s := range statics {
		available[s.Name] = struct{}{}
	}

	for _, c := range calls {
		for _, name := range c.References() {
			if _, ok := available[name]; ok {
				continue
			}
			if from, ok := producer[name]; ok {
				msg := fmt.Sprintf("produced by call %q, which runs after it", from)
				if from == c.Name {
					msg = "a call cannot use a variable it extracts itself"
				}
				return &ValidationError{Call: c.Name, Variable: name, Message: msg, Kind: ErrVarOutOfOrder}
			}
			return &ValidationError{Call: c.Name, Variable: name, Message: "unknown variable", Kind: ErrUnknownVar}
		}
		for _, e := range c.Extractions {
			available[e.Name] = struct{}{}
		}
	}
	return nil
}

// LocalVariableNames returns the static and call variable names of the workflow.
func (w *Workflow) LocalVariableNames() []string {
	names := make([]string, 0, len(w.Statics))
	for _, s := range w.Statics {
		names = append(names, s.Name)
	}
	for _, c := range w.Calls {
		for _, e := range c.Extractions {
			names = append(names, e.Name)
		}
	}
	return names
}

// UsesVariable reports whether any call references name.
func (w *Workflow) UsesVariable(name string) bool {
	for _, c := range w.Calls {
		for _, ref := range c.References() {
			if ref == name {
				return true
			}
		}
	}
	return false
}
