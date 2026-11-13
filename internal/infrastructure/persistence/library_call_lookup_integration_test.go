//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
)

// A call can be saved to the library from any workflow the user owns, and never
// from another user's workflow.
func TestLibrarySaveFromCall_AcrossWorkflows(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	owner := createUser(t, db)
	other := createUser(t, db)

	key, _ := crypto.GenerateSecretKey()
	cipher, err := crypto.NewAESCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	workflows := write.NewWorkflowWriteRepository(db, cipher)
	library := write.NewLibraryWriteRepository(db)

	w, err := domainworkflow.New(owner, domainworkflow.Definition{
		Name: "Checkout", Interval: 5 * time.Minute, Enabled: true,
		Calls: []domaincall.Definition{{Name: "login", Method: "POST", URL: "https://api.example.com/login"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflows.Save(ctx, w); err != nil {
		t.Fatal(err)
	}
	callID := w.Calls[0].ID

	gotWF, err := workflows.FindWorkflowIDByCall(ctx, owner, callID)
	if err != nil || gotWF != w.ID {
		t.Fatalf("owner lookup: %v %v want %v", gotWF, err, w.ID)
	}
	if _, err := workflows.FindWorkflowIDByCall(ctx, other, callID); !errors.Is(err, domainworkflow.ErrCallNotFound) {
		t.Fatalf("other user lookup must be ErrCallNotFound, got %v", err)
	}
	if _, err := workflows.FindWorkflowIDByCall(ctx, owner, uuid.New()); !errors.Is(err, domainworkflow.ErrCallNotFound) {
		t.Fatalf("unknown call must be ErrCallNotFound, got %v", err)
	}

	loaded, err := workflows.GetByID(ctx, gotWF)
	if err != nil || loaded == nil {
		t.Fatalf("load workflow: %v", err)
	}
	def, err := loaded.CallDefinition(callID)
	if err != nil {
		t.Fatal(err)
	}
	item, err := domainlibrary.New(owner, def, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := library.Save(ctx, &item); err != nil {
		t.Fatalf("save item: %v", err)
	}
	stored, err := library.FindByID(ctx, owner, item.ID)
	if err != nil || stored == nil || stored.Definition.URL != "https://api.example.com/login" {
		t.Fatalf("stored item: %+v %v", stored, err)
	}
}
