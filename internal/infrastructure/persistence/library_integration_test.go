//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/domain/paginate"
	domainrule "go-api/internal/domain/rule"
	"go-api/internal/infrastructure/persistence/read"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
)

func libraryDefinition(name string) domaincall.Definition {
	verifyTLS := false
	return domaincall.Definition{
		Name:        name,
		Description: "probe",
		Method:      "POST",
		URL:         "https://api.example.com/{{base_url}}/login",
		Headers:     map[string]string{"Content-Type": "application/json"},
		Query:       map[string]string{"scope": "read"},
		Body:        `{"user":"demo"}`,
		Timeout:     7 * time.Second,
		VerifyTLS:   &verifyTLS,
		IPVersion:   domaincall.IPv4,
		ProxyURL:    "http://proxy.example.com:8080",
		Rules: []domainrule.Rule{
			{Subject: domainrule.SubjectStatus, Operator: domainrule.OpIs, Value: "200"},
		},
	}
}

func TestLibraryRepository_RoundTrip(t *testing.T) {
	db := openDB(t)
	userID := createUser(t, db)
	repo := write.NewLibraryWriteRepository(db)

	item, err := domainlibrary.New(userID, libraryDefinition("Login"), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), &item); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.FindByID(context.Background(), userID, item.ID)
	if err != nil || got == nil {
		t.Fatalf("find: %v %v", got, err)
	}
	d := got.Definition
	if d.Name != "Login" || d.Method != "POST" || d.Timeout != 7*time.Second || d.IPVersion != domaincall.IPv4 {
		t.Fatalf("scalar fields lost: %+v", d)
	}
	if d.VerifyTLS == nil || *d.VerifyTLS != false || d.ProxyURL == "" || d.Headers["Content-Type"] != "application/json" {
		t.Fatalf("optional fields lost: %+v", d)
	}
	if len(d.Rules) != 1 || d.Rules[0].Value != "200" {
		t.Fatalf("rules lost: %+v", d.Rules)
	}
}

func TestLibraryRepository_OwnershipAndDelete(t *testing.T) {
	db := openDB(t)
	owner := createUser(t, db)
	other := createUser(t, db)
	repo := write.NewLibraryWriteRepository(db)
	ctx := context.Background()

	item, err := domainlibrary.New(owner, libraryDefinition("Owned"), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, &item); err != nil {
		t.Fatal(err)
	}

	if got, err := repo.FindByID(ctx, other, item.ID); err != nil || got != nil {
		t.Fatalf("other user must not see the item: %v %v", got, err)
	}
	if err := repo.Delete(ctx, other, item.ID); !errors.Is(err, domainlibrary.ErrNotFound) {
		t.Fatalf("other user delete: expected ErrNotFound, got %v", err)
	}
	if err := repo.Delete(ctx, owner, item.ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if err := repo.Delete(ctx, owner, item.ID); !errors.Is(err, domainlibrary.ErrNotFound) {
		t.Fatalf("second delete: expected ErrNotFound, got %v", err)
	}
}

func TestLibraryRepository_UpdateKeepsIdentity(t *testing.T) {
	db := openDB(t)
	userID := createUser(t, db)
	repo := write.NewLibraryWriteRepository(db)
	ctx := context.Background()

	item, err := domainlibrary.New(userID, libraryDefinition("Before"), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, &item); err != nil {
		t.Fatal(err)
	}
	if err := item.Replace(libraryDefinition("After"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, &item); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := repo.FindByID(ctx, userID, item.ID)
	if err != nil || got == nil || got.Definition.Name != "After" {
		t.Fatalf("update not persisted: %+v %v", got, err)
	}
}

func TestLibraryQueryRepository_ListSearchSortPaginate(t *testing.T) {
	db := openDB(t)
	userID := createUser(t, db)
	repo := write.NewLibraryWriteRepository(db)
	reads := read.NewLibraryQueryRepository(db)
	ctx := context.Background()

	for _, name := range []string{"Alpha health", "Beta login", "gamma HEALTH"} {
		item, err := domainlibrary.New(userID, libraryDefinition(name), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, &item); err != nil {
			t.Fatal(err)
		}
	}

	views, total, err := reads.List(ctx, userID, paginate.PaginateQuery{Page: 1, Limit: 10, SortBy: "name", OrderBy: "asc", Search: "health"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(views) != 2 {
		t.Fatalf("search: total %d rows %d", total, len(views))
	}
	if views[0].Name != "Alpha health" || views[0].Method != "POST" || views[0].URL == "" {
		t.Fatalf("unexpected first row %+v", views[0])
	}

	page, total, err := reads.List(ctx, userID, paginate.PaginateQuery{Page: 2, Limit: 2, SortBy: "name", OrderBy: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(page) != 1 || page[0].Name != "gamma HEALTH" {
		t.Fatalf("page 2: total %d rows %+v", total, page)
	}

	none, total, err := reads.List(ctx, uuid.New(), paginate.PaginateQuery{Page: 1, Limit: 10})
	if err != nil || total != 0 || len(none) != 0 {
		t.Fatalf("other user must see nothing: %d %d %v", total, len(none), err)
	}
}
