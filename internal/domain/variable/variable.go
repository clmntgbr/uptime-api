package variable

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeGlobal Type = "global"
	TypeStatic Type = "static"
	TypeCall Type = "call"
)

const (
	BuiltinTimestamp = "timestamp"
	BuiltinUUID      = "uuid"
	BuiltinRunID     = "run_id"

	MaxValueBytes = 4 * 1024
	MaxNameLength = 64

	MaskedValue = "••••"
)

var (
	namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

	ErrInvalidName   = errors.New("invalid variable name")
	ErrReservedName  = errors.New("reserved variable name")
	ErrValueTooLarge = errors.New("variable value exceeds 4 KB")
	ErrNotFound      = errors.New("variable not found")
	ErrAlreadyExists = errors.New("variable already exists")
)

type Available struct {
	Name     string
	Type     Type
	Path     string
	Source   string
	CallID   uuid.UUID
	CallName string
	Secret   bool
	Value    string
}

func Builtins() []string {
	return []string{BuiltinTimestamp, BuiltinUUID, BuiltinRunID}
}

func IsBuiltin(name string) bool {
	switch name {
	case BuiltinTimestamp, BuiltinUUID, BuiltinRunID:
		return true
	}
	return false
}

func ValidateName(name string) error {
	if len(name) == 0 || len(name) > MaxNameLength || !namePattern.MatchString(name) {
		return fmt.Errorf("%w %q: use letters, digits, underscores and hyphens, starting with a letter or underscore (max %d)",
			ErrInvalidName, name, MaxNameLength)
	}
	if IsBuiltin(name) {
		return fmt.Errorf("%w %q", ErrReservedName, name)
	}
	return nil
}

func ValidateValue(name, value string) error {
	if len(value) > MaxValueBytes {
		return fmt.Errorf("%w: %q", ErrValueTooLarge, name)
	}
	return nil
}

type Global struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Value     string
	Secret    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewGlobal(userID uuid.UUID, name, value string, secret bool) (*Global, error) {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	if err := ValidateValue(name, value); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Global{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		Value:     value,
		Secret:    secret,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (g *Global) Change(value string, secret bool) error {
	if err := ValidateValue(g.Name, value); err != nil {
		return err
	}
	g.Value = value
	g.Secret = secret
	g.UpdatedAt = time.Now().UTC()
	return nil
}
