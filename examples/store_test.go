package examples

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := OpenStore(context.Background(), SQLiteConfig(filepath.Join(t.TempDir(), "examples.db")))
	if err != nil {
		t.Fatalf("OpenStore() = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.CreateSchema(context.Background()); err != nil {
		t.Fatalf("CreateSchema() = %v", err)
	}
	return store
}

func TestStoreSelects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)

	alice := &User{Name: "alice", Email: "alice@exemplo.dev", Status: StatusActive}
	if err := store.CreateUserWithFirstTasks(ctx, alice, "escrever a spec", "revisar o PR"); err != nil {
		t.Fatalf("CreateUserWithFirstTasks(alice) = %v", err)
	}
	carol := &User{Name: "carol", Email: "carol@exemplo.dev", Status: StatusInactive}
	if err := store.CreateUser(ctx, carol); err != nil {
		t.Fatalf("CreateUser(carol) = %v", err)
	}

	users, err := store.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() = %v", err)
	}
	if len(users) != 2 || users[0].Name != "alice" || users[1].Name != "carol" {
		t.Errorf("ListUsers() = %+v", users)
	}

	one, err := store.UserByEmail(ctx, alice.Email)
	if err != nil {
		t.Fatalf("UserByEmail() = %v", err)
	}
	if one.ID != alice.ID || one.Status != StatusActive {
		t.Errorf("UserByEmail() = %+v", one)
	}

	if _, err := store.UserByID(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UserByID(999) = %v, want sql.ErrNoRows", err)
	}

	count, err := store.CountOpenTasks(ctx, alice.ID)
	if err != nil {
		t.Fatalf("CountOpenTasks() = %v", err)
	}
	if count != 2 {
		t.Errorf("CountOpenTasks() = %d, want 2", count)
	}
}

func TestStoreJoinsAndPagination(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)

	alice := &User{Name: "alice", Email: "alice@exemplo.dev", Status: StatusActive}
	if err := store.CreateUserWithFirstTasks(ctx, alice, "escrever a spec", "revisar o PR"); err != nil {
		t.Fatalf("CreateUserWithFirstTasks(alice) = %v", err)
	}
	carol := &User{Name: "carol", Email: "carol@exemplo.dev", Status: StatusActive}
	if err := store.CreateUserWithFirstTasks(ctx, carol, "publicar"); err != nil {
		t.Fatalf("CreateUserWithFirstTasks(carol) = %v", err)
	}
	dave := &User{Name: "dave", Email: "dave@exemplo.dev", Status: StatusActive}
	if err := store.CreateUser(ctx, dave); err != nil {
		t.Fatalf("CreateUser(dave) = %v", err)
	}

	open, err := store.ListOpenTasksByOwnerEmail(ctx, alice.Email)
	if err != nil {
		t.Fatalf("ListOpenTasksByOwnerEmail() = %v", err)
	}
	if len(open) != 2 {
		t.Errorf("ListOpenTasksByOwnerEmail(alice) = %d tasks, want 2", len(open))
	}

	counts, err := store.ListUserTaskCounts(ctx)
	if err != nil {
		t.Fatalf("ListUserTaskCounts() = %v", err)
	}
	wantCounts := []UserTaskCount{
		{ID: 1, Name: "alice", Tasks: 2},
		{ID: 2, Name: "carol", Tasks: 1},
		{ID: 3, Name: "dave", Tasks: 0},
	}
	if len(counts) != len(wantCounts) {
		t.Fatalf("ListUserTaskCounts() = %+v, want %+v", counts, wantCounts)
	}
	for i := range wantCounts {
		if counts[i] != wantCounts[i] {
			t.Errorf("count row %d = %+v, want %+v", i, counts[i], wantCounts[i])
		}
	}

	page, info, err := store.ListUsersPage(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListUsersPage() = %v", err)
	}
	if info.Total != 3 || info.Pages != 2 || info.Page != 1 || !info.HasNext || info.HasPrev {
		t.Errorf("page info = %+v", info)
	}
	if len(page) != 2 || page[0].Name != "alice" || page[1].Name != "carol" {
		t.Errorf("page rows = %+v", page)
	}

	moved, err := store.TransferTasks(ctx, alice.ID, dave.ID)
	if err != nil {
		t.Fatalf("TransferTasks() = %v", err)
	}
	if moved != 2 {
		t.Errorf("TransferTasks() = %d, want 2", moved)
	}
}
