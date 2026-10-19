package tests

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

// setupGroupSchema creates the related tables used by the group tests:
// accounts (1) -> entries (N).
func setupGroupSchema(t *testing.T, db *krios.DB) {
	t.Helper()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE accounts (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatalf("CREATE accounts = %v", err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE entries (id INTEGER PRIMARY KEY, account_id INTEGER NOT NULL, amount INTEGER NOT NULL)"); err != nil {
		t.Fatalf("CREATE entries = %v", err)
	}
}

func countRows(t *testing.T, db *krios.DB, table string) int {
	t.Helper()

	ctx := context.Background()
	rows, err := db.QueryContext(ctx, "SELECT COUNT(*) FROM "+table)
	if err != nil {
		t.Fatalf("QueryContext(%s) = %v", table, err)
	}
	defer rows.Close()

	var n int
	if !rows.Next() {
		t.Fatalf("Next(%s) = false, want true", table)
	}
	if err := rows.Scan(&n); err != nil {
		t.Fatalf("Scan(%s) = %v", table, err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("Close(%s) = %v", table, err)
	}
	return n
}

func insertGroup(t *testing.T, tx *krios.Tx) {
	t.Helper()

	ctx := context.Background()
	if _, err := tx.ExecContext(ctx, "INSERT INTO accounts (id, name) VALUES (?, ?)", 1, "main"); err != nil {
		t.Fatalf("tx INSERT account = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO entries (id, account_id, amount) VALUES (?, ?, ?)", 1, 1, 100); err != nil {
		t.Fatalf("tx INSERT entry 1 = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO entries (id, account_id, amount) VALUES (?, ?, ?)", 2, 1, 250); err != nil {
		t.Fatalf("tx INSERT entry 2 = %v", err)
	}
}

func TestGroupOfThreeInsertsCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	insertGroup(t, tx)
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

func TestGroupRollsBackWhenOneInsertFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	insertGroup(t, tx)

	// Third statement fails: primary key of accounts already taken.
	if _, err := tx.ExecContext(ctx, "INSERT INTO accounts (id, name) VALUES (?, ?)", 1, "dup"); err == nil {
		t.Fatalf("tx INSERT duplicate = nil, want constraint error")
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

func TestCommitBlockedAfterStatementFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	insertGroup(t, tx)

	// The failing statement: its error must be the one returned by Commit.
	execErr := func() error {
		_, err := tx.ExecContext(ctx, "INSERT INTO accounts (id, name) VALUES (?, ?)", 1, "dup")
		return err
	}()
	if execErr == nil {
		t.Fatalf("tx INSERT duplicate = nil, want constraint error")
	}

	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Fatalf("Commit() = nil, want error blocked by previous failure")
	}
	if !errors.Is(commitErr, execErr) {
		t.Fatalf("Commit() = %v, want errors.Is(_, %v)", commitErr, execErr)
	}

	// Auto-rollback happened: nothing from the group persists.
	if n := countRows(t, db, "accounts"); n != 0 {
		t.Fatalf("accounts = %d, want 0 (auto-rollback)", n)
	}
	if n := countRows(t, db, "entries"); n != 0 {
		t.Fatalf("entries = %d, want 0 (auto-rollback)", n)
	}

	// Transaction is done after the auto-rollback.
	if err := tx.Commit(ctx); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("second Commit() = %v, want ErrTxDone", err)
	}
	if err := tx.Rollback(ctx); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("Rollback() after blocked Commit = %v, want ErrTxDone", err)
	}
}

func TestOperationsOutsideGroupAreIndependent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	// Confirmed before the group starts.
	if _, err := db.ExecContext(ctx, "INSERT INTO accounts (id, name) VALUES (?, ?)", 100, "external-before"); err != nil {
		t.Fatalf("db INSERT before = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO accounts (id, name) VALUES (?, ?)", 1, "group"); err != nil {
		t.Fatalf("tx INSERT = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO entries (id, account_id, amount) VALUES (?, ?, ?)", 1, 1, 10); err != nil {
		t.Fatalf("tx INSERT = %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() = %v", err)
	}

	// Confirmed after the group was reverted.
	if _, err := db.ExecContext(ctx, "INSERT INTO accounts (id, name) VALUES (?, ?)", 101, "external-after"); err != nil {
		t.Fatalf("db INSERT after = %v", err)
	}

	if n := countRows(t, db, "accounts"); n != 2 {
		t.Fatalf("accounts = %d, want 2 (only external ones)", n)
	}
	if n := countRows(t, db, "entries"); n != 0 {
		t.Fatalf("entries = %d, want 0 (group reverted)", n)
	}
}

func TestDBReadsWhileTxOpen(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO accounts (id, name) VALUES (?, ?)", 1, "uncommitted"); err != nil {
		t.Fatalf("tx INSERT = %v", err)
	}

	// The main instance must not see uncommitted transaction data.
	if n := countRows(t, db, "accounts"); n != 0 {
		t.Fatalf("accounts during open tx = %d, want 0", n)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() = %v", err)
	}

	if n := countRows(t, db, "accounts"); n != 0 {
		t.Fatalf("accounts after rollback = %d, want 0", n)
	}
}

func TestTxRejectsAllMethodsAfterDone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cases := []struct {
		name string
		end  func(tx *krios.Tx) error
	}{
		{"after Commit", func(tx *krios.Tx) error { return tx.Commit(ctx) }},
		{"after Rollback", func(tx *krios.Tx) error { return tx.Rollback(ctx) }},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := openTestDB(t)
			setupGroupSchema(t, db)

			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatalf("Begin() = %v", err)
			}
			if err := tc.end(tx); err != nil {
				t.Fatalf("end = %v", err)
			}

			if _, err := tx.ExecContext(ctx, "SELECT 1"); !errors.Is(err, krios.ErrTxDone) {
				t.Fatalf("ExecContext() = %v, want ErrTxDone", err)
			}
			if _, err := tx.QueryContext(ctx, "SELECT 1"); !errors.Is(err, krios.ErrTxDone) {
				t.Fatalf("QueryContext() = %v, want ErrTxDone", err)
			}
			var v int
			if err := tx.QueryRowContext(ctx, "SELECT 1").Scan(&v); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("QueryRowContext().Scan() = %v, want sql.ErrNoRows", err)
			}
			if err := tx.Commit(ctx); !errors.Is(err, krios.ErrTxDone) {
				t.Fatalf("Commit() = %v, want ErrTxDone", err)
			}
			if err := tx.Rollback(ctx); !errors.Is(err, krios.ErrTxDone) {
				t.Fatalf("Rollback() = %v, want ErrTxDone", err)
			}
		})
	}
}

func TestConcurrentCallsOnSameTxSerialized(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	const workers = 8

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tx.ExecContext(ctx,
				"INSERT INTO entries (id, account_id, amount) VALUES (?, ?, ?)", i, 0, 1)
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent tx ExecContext() = %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}
	if n := countRows(t, db, "entries"); n != workers {
		t.Fatalf("entries = %d, want %d", n, workers)
	}
}

func TestConcurrentDBAndDistinctTx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	setupGroupSchema(t, db)

	const rounds = 20

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	// Main instance reads concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range rounds {
			rows, err := db.QueryContext(ctx, "SELECT COUNT(*) FROM accounts")
			if err != nil {
				errCh <- err
				return
			}
			var n int
			if rows.Next() {
				if err := rows.Scan(&n); err != nil {
					_ = rows.Close()
					errCh <- err
					return
				}
			}
			if err := rows.Close(); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Two distinct transactions read and roll back concurrently.
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := db.Begin(ctx)
			if err != nil {
				errCh <- err
				return
			}
			for range rounds {
				rows, err := tx.QueryContext(ctx, "SELECT COUNT(*) FROM entries")
				if err != nil {
					errCh <- err
					return
				}
				var n int
				if rows.Next() {
					if err := rows.Scan(&n); err != nil {
						_ = rows.Close()
						errCh <- err
						return
					}
				}
				if err := rows.Close(); err != nil {
					errCh <- err
					return
				}
			}
			if err := tx.Rollback(ctx); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent operation = %v", err)
	}
}
