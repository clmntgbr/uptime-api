package library

import (
	"errors"
	"time"

	domaincall "go-api/internal/domain/call"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("library item not found")
	// ErrImportTooLarge is returned when the import content exceeds MaxImportBytes.
	ErrImportTooLarge = errors.New("import content exceeds 2 MB")
	// ErrUnsupportedFormat is returned for an import format the parser does not handle.
	ErrUnsupportedFormat = errors.New("unsupported import format")
)

// MaxImportBytes caps the raw content of one import request.
const MaxImportBytes = 2 * 1024 * 1024

// MaxImportItems caps how many candidates one confirmation can create.
const MaxImportItems = 50

// Item is a reusable call template owned by one user. Inserting an item copies
// its definition into a workflow; later changes on either side are independent.
type Item struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Definition domaincall.Definition
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Normalize validates def with the same rules as a call and returns its
// normalized form (default method, timeout, IP version, rule IDs cleared).
func Normalize(def domaincall.Definition) (domaincall.Definition, error) {
	call, err := domaincall.New(def, 1, uuid.New())
	if err != nil {
		return domaincall.Definition{}, err
	}
	return call.Definition(), nil
}

// New validates def and builds a new item owned by userID.
func New(userID uuid.UUID, def domaincall.Definition, now time.Time) (Item, error) {
	normalized, err := Normalize(def)
	if err != nil {
		return Item{}, err
	}
	return Item{
		ID:         uuid.New(),
		UserID:     userID,
		Definition: normalized,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// Replace validates def and replaces the item definition. Only the definition
// changes; the identity and creation date are kept.
func (i *Item) Replace(def domaincall.Definition, now time.Time) error {
	normalized, err := Normalize(def)
	if err != nil {
		return err
	}
	i.Definition = normalized
	i.UpdatedAt = now
	return nil
}

// ListView is one row of GET /library.
type ListView struct {
	ID          uuid.UUID
	Name        string
	Description string
	Method      string
	URL         string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
