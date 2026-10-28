package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/event"
	"go-api/internal/domain/schedule"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

const (
	MaxCalls        = 20
	MaxVariables    = 50
	MaxNameLength   = 200
	MinInterval     = schedule.Tick
	WorkflowTimeout = 5 * time.Minute
)

type StaticVariable struct {
	ID     uuid.UUID
	Name   string
	Value  string
	Secret bool
}

// Definition is the user input to create or replace a workflow.
type Definition struct {
	Name         string
	ScheduleType ScheduleType
	Interval     time.Duration // required for interval; zero otherwise
	RunAt        *time.Time    // required for at; nil otherwise
	Enabled      bool
	Statics      []StaticVariable
	Calls        []domaincall.Definition
}

// Workflow is an ordered list of calls. Scheduling is interval, once, or at a
// fixed time. Calls only see the variables produced before them.
type Workflow struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Name         string
	ScheduleType ScheduleType
	Interval     time.Duration
	RunAt        *time.Time
	Timeout      time.Duration
	Enabled      bool
	Statics      []StaticVariable
	Calls        []domaincall.Call
	CreatedAt    time.Time
	UpdatedAt    time.Time

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
	w.recordEvent(w.newUpdatedOrCreatedEvent(true, now))
	return w, nil
}

// Update replaces the definition. When Calls are replaced, IDs are kept by
// position so results stay attached across edits. It reports whether the
// schedule changed (type, interval, runAt or enablement).
func (w *Workflow) Update(def Definition, globalNames []string) (bool, error) {
	previous := scheduleSnapshot(w)
	if err := w.apply(def, globalNames); err != nil {
		return false, err
	}
	w.UpdatedAt = time.Now().UTC()
	w.recordEvent(w.newUpdatedOrCreatedEvent(false, w.UpdatedAt))
	return previous != scheduleSnapshot(w), nil
}

// Disable turns the workflow off and records WorkflowUpdated. Returns false when
// it was already disabled.
func (w *Workflow) Disable() bool {
	if !w.Enabled {
		return false
	}
	w.Enabled = false
	w.UpdatedAt = time.Now().UTC()
	w.recordEvent(w.newUpdatedOrCreatedEvent(false, w.UpdatedAt))
	return true
}

type scheduleKey struct {
	Type     ScheduleType
	Interval time.Duration
	RunAt    int64
	Enabled  bool
}

func scheduleSnapshot(w *Workflow) scheduleKey {
	var runAt int64
	if w.RunAt != nil {
		runAt = w.RunAt.Unix()
	}
	return scheduleKey{Type: w.ScheduleType, Interval: w.Interval, RunAt: runAt, Enabled: w.Enabled}
}

func (w *Workflow) newUpdatedOrCreatedEvent(created bool, at time.Time) event.DomainEvent {
	intervalSeconds := 0
	if w.ScheduleType == ScheduleInterval {
		intervalSeconds = int(w.Interval / time.Second)
	}
	var runAt *time.Time
	if w.RunAt != nil {
		t := w.RunAt.UTC()
		runAt = &t
	}
	if created {
		return WorkflowCreated{
			ID:              uuid.New().String(),
			WorkflowID:      w.ID.String(),
			UserID:          w.UserID.String(),
			Name:            w.Name,
			ScheduleType:    string(w.ScheduleType),
			IntervalSeconds: intervalSeconds,
			RunAt:           runAt,
			Enabled:         w.Enabled,
			CallCount:       len(w.Calls),
			Timestamp:       at,
		}
	}
	return WorkflowUpdated{
		ID:              uuid.New().String(),
		WorkflowID:      w.ID.String(),
		UserID:          w.UserID.String(),
		Name:            w.Name,
		ScheduleType:    string(w.ScheduleType),
		IntervalSeconds: intervalSeconds,
		RunAt:           runAt,
		Enabled:         w.Enabled,
		CallCount:       len(w.Calls),
		Timestamp:       at,
	}
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
	scheduleType, interval, runAt, err := normalizeSchedule(def, w.RunAt)
	if err != nil {
		return err
	}
	if len(def.Calls) > MaxCalls {
		return &ValidationError{Message: fmt.Sprintf("a workflow has at most %d calls", MaxCalls)}
	}

	// Empty Statics/Calls keep the current lists: HTTP create/update only touch
	// metadata. Dedicated endpoints (and YAML import) pass the values to replace.
	statics := w.Statics
	if len(def.Statics) > 0 {
		built, err := buildStatics(w.Statics, def.Statics)
		if err != nil {
			return err
		}
		statics = built
	}

	calls := w.Calls
	if len(def.Calls) > 0 {
		calls = make([]domaincall.Call, 0, len(def.Calls))
		for i, cd := range def.Calls {
			id := uuid.New()
			var prevExtractions []domaincall.Extraction
			if i < len(w.Calls) {
				id = w.Calls[i].ID
				prevExtractions = w.Calls[i].Extractions
			}
			cd = withPreservedExtractionIDs(cd, prevExtractions)
			call, err := domaincall.New(cd, i+1, id)
			if err != nil {
				var cerr *domaincall.ValidationError
				if errors.As(err, &cerr) {
					return &ValidationError{Call: cerr.Call, Message: cerr.Message}
				}
				return err
			}
			calls = append(calls, call)
		}
	}

	if err := validateVariables(statics, calls, globalNames); err != nil {
		return err
	}

	w.Name = name
	w.ScheduleType = scheduleType
	w.Interval = interval
	w.RunAt = runAt
	w.Enabled = def.Enabled
	w.Statics = statics
	w.Calls = calls
	return nil
}

func normalizeSchedule(def Definition, previousRunAt *time.Time) (ScheduleType, time.Duration, *time.Time, error) {
	typ := def.ScheduleType
	if typ == "" {
		typ = ScheduleInterval
	}
	switch typ {
	case ScheduleInterval:
		if def.Interval < MinInterval || def.Interval%schedule.Tick != 0 {
			return "", 0, nil, &ValidationError{Message: "interval must be a whole number of minutes, at least 1 minute"}
		}
		if def.RunAt != nil {
			return "", 0, nil, &ValidationError{Message: "runAt is only valid for schedule type at"}
		}
		return typ, def.Interval, nil, nil
	case ScheduleOnce:
		if def.Interval != 0 {
			return "", 0, nil, &ValidationError{Message: "intervalSeconds is only valid for schedule type interval"}
		}
		if def.RunAt != nil {
			return "", 0, nil, &ValidationError{Message: "runAt is only valid for schedule type at"}
		}
		return typ, 0, nil, nil
	case ScheduleAt:
		if def.Interval != 0 {
			return "", 0, nil, &ValidationError{Message: "intervalSeconds is only valid for schedule type interval"}
		}
		if def.RunAt == nil {
			return "", 0, nil, &ValidationError{Message: "runAt is required for schedule type at"}
		}
		at := schedule.AlignAt(*def.RunAt)
		unchanged := previousRunAt != nil && at.Equal(schedule.AlignAt(*previousRunAt))
		if !unchanged && !at.After(time.Now().UTC()) {
			return "", 0, nil, &ValidationError{Message: "runAt must be a future minute"}
		}
		return typ, 0, &at, nil
	default:
		return "", 0, nil, &ValidationError{Message: "scheduleType must be interval, once or at"}
	}
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
func validateVariables(statics []StaticVariable, calls []domaincall.Call, globalNames []string) error {
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
				if from == c.Name {
					return &ValidationError{
						Call:         c.Name,
						Variable:     name,
						ConflictCall: from,
						Reason:       ReasonSelfReference,
						Message:      "a call cannot use a variable it extracts itself",
						Kind:         ErrVarOutOfOrder,
					}
				}
				return &ValidationError{
					Call:         c.Name,
					Variable:     name,
					ConflictCall: from,
					Reason:       ReasonProducedByLaterCall,
					Message:      fmt.Sprintf("produced by call %q, which runs after it", from),
					Kind:         ErrVarOutOfOrder,
				}
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
