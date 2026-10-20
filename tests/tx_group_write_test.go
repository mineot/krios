package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/mineot/krios"
)

func TestGroupWritesWithBuildersCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	entAcc, err := krios.EntityOf[groupAccount]()
	if err != nil {
		t.Fatalf("EntityOf[groupAccount]() = %v", err)
	}
	entEnt, err := krios.EntityOf[groupEntry]()
	if err != nil {
		t.Fatalf("EntityOf[groupEntry]() = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Insert(entAcc).SetValues(&groupAccount{ID: 1, Name: "main"}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert account = %v", err)
	}
	if _, err := tx.Insert(entEnt).SetValues(&groupEntry{ID: 1, AccountID: 1, Amount: 100}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert entry 1 = %v", err)
	}
	if _, err := tx.Insert(entEnt).SetValues(&groupEntry{ID: 2, AccountID: 1, Amount: 250}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert entry 2 = %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}

	if n := countRows(t, db, "accounts"); n != 1 {
		t.Fatalf("accounts = %d, want 1", n)
	}
	if n := countRows(t, db, "entries"); n != 2 {
		t.Fatalf("entries = %d, want 2", n)
	}
}

func TestGroupWritesWithBuildersRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	entAcc, err := krios.EntityOf[groupAccount]()
	if err != nil {
		t.Fatalf("EntityOf[groupAccount]() = %v", err)
	}
	entEnt, err := krios.EntityOf[groupEntry]()
	if err != nil {
		t.Fatalf("EntityOf[groupEntry]() = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Insert(entAcc).SetValues(&groupAccount{ID: 1, Name: "main"}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert account = %v", err)
	}
	if _, err := tx.Insert(entEnt).SetValues(&groupEntry{ID: 1, AccountID: 1, Amount: 100}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert entry 1 = %v", err)
	}
	if _, err := tx.Insert(entEnt).SetValues(&groupEntry{ID: 2, AccountID: 1, Amount: 250}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert entry 2 = %v", err)
	}

	if _, err := tx.Insert(entAcc).SetValues(&groupAccount{ID: 1, Name: "dup"}).Exec(ctx); err == nil {
		t.Fatalf("tx Insert duplicate = nil, want constraint error")
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() = %v", err)
	}

	if n := countRows(t, db, "accounts"); n != 0 {
		t.Fatalf("accounts = %d, want 0", n)
	}
	if n := countRows(t, db, "entries"); n != 0 {
		t.Fatalf("entries = %d, want 0", n)
	}
}

func TestGroupWritesWithBuildersCommitBlocked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	entAcc, err := krios.EntityOf[groupAccount]()
	if err != nil {
		t.Fatalf("EntityOf[groupAccount]() = %v", err)
	}
	entEnt, err := krios.EntityOf[groupEntry]()
	if err != nil {
		t.Fatalf("EntityOf[groupEntry]() = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Insert(entAcc).SetValues(&groupAccount{ID: 1, Name: "main"}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert account = %v", err)
	}
	if _, err := tx.Insert(entEnt).SetValues(&groupEntry{ID: 1, AccountID: 1, Amount: 100}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert entry 1 = %v", err)
	}
	if _, err := tx.Insert(entEnt).SetValues(&groupEntry{ID: 2, AccountID: 1, Amount: 250}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert entry 2 = %v", err)
	}

	execErr := func() error {
		_, err := tx.Insert(entAcc).SetValues(&groupAccount{ID: 1, Name: "dup"}).Exec(ctx)
		return err
	}()
	if execErr == nil {
		t.Fatalf("tx Insert duplicate = nil, want constraint error")
	}

	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Fatalf("Commit() = nil, want error blocked by previous failure")
	}
	if !errors.Is(commitErr, execErr) {
		t.Fatalf("Commit() = %v, want errors.Is(_, %v)", commitErr, execErr)
	}

	if n := countRows(t, db, "accounts"); n != 0 {
		t.Fatalf("accounts = %d, want 0 (auto-rollback)", n)
	}
	if n := countRows(t, db, "entries"); n != 0 {
		t.Fatalf("entries = %d, want 0 (auto-rollback)", n)
	}

	if err := tx.Commit(ctx); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("second Commit() = %v, want ErrTxDone", err)
	}
	if err := tx.Rollback(ctx); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("Rollback() after blocked Commit = %v, want ErrTxDone", err)
	}
}

type groupAccount struct {
	ID   int64  `krios:"id,pk"`
	Name string `krios:"name"`
}

func (groupAccount) TableName() string { return "accounts" }

type groupEntry struct {
	ID        int64 `krios:"id,pk"`
	AccountID int64 `krios:"account_id"`
	Amount    int64 `krios:"amount"`
}

func (groupEntry) TableName() string { return "entries" }
