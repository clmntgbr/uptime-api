//go:build integration

package persistence_test

import (
	"context"
	"fmt"
	"testing"

	cmdlibrary "go-api/internal/application/command/library"
	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/infrastructure/persistence/outbox"
	"go-api/internal/infrastructure/persistence/read"
	"go-api/internal/infrastructure/persistence/write"
)

// A mass import of many candidates is stored in one transaction.
func TestLibraryMassImport_OneTransaction(t *testing.T) {
	db := openDB(t)
	userID := createUser(t, db)
	repo := write.NewLibraryWriteRepository(db)
	reads := read.NewLibraryQueryRepository(db)

	const count = 120
	items := make([]cmdlibrary.ImportItem, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, cmdlibrary.ImportItem{
			Key: fmt.Sprintf("op-%d", i),
			Definition: domaincall.Definition{
				Name: fmt.Sprintf("call %03d", i), Method: "GET",
				URL: fmt.Sprintf("https://api.example.com/items/%d", i),
			},
		})
	}
	created, err := cmdlibrary.NewImportHandler(repo, outbox.NewRepository(db)).Handle(context.Background(), cmdlibrary.ImportCommand{
		UserID: userID, Items: items,
	})
	if err != nil {
		t.Fatalf("mass import: %v", err)
	}
	if len(created) != count {
		t.Fatalf("created %d want %d", len(created), count)
	}

	_, total, err := reads.List(context.Background(), userID, domainlibrary.ListFilter{})
	if err != nil || total != count {
		t.Fatalf("stored %d want %d (%v)", total, count, err)
	}
}
