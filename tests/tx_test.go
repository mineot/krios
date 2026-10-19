package tests

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *krios.DB {
	t.Helper()

	cfg := sqliteConfig(filepath.Join(t.TempDir(), "tx.db"))
	db, err := krios.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestBeginCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v, want nil", err)
	}
	defer tx.Rollback(context.Background())
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v, want nil", err)
	}
}

func TestBeginRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v, want nil", err)
	}
	defer tx.Rollback(context.Background())
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() = %v, want nil", err)
	}
}

func TestTransactionPersistsDataOnCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	if _, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.ExecContext(ctx, "INSERT INTO t (v) VALUES (?)", "a"); err != nil {
		t.Fatalf("tx INSERT = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO t (v) VALUES (?)", "b"); err != nil {
		t.Fatalf("tx INSERT = %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT COUNT(*) FROM t")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	defer rows.Close()

	var count int
	if !rows.Next() {
		t.Fatalf("Next() = false, want true")
	}
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestTransactionDiscardsDataOnRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	if _, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.ExecContext(ctx, "INSERT INTO t (v) VALUES (?)", "x"); err != nil {
		t.Fatalf("tx INSERT = %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() = %v", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT COUNT(*) FROM t")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	defer rows.Close()

	var count int
	if !rows.Next() {
		t.Fatalf("Next() = false, want true")
	}
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

func TestTxRejectsOperationsAfterDone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}

	if _, err := tx.ExecContext(ctx, "SELECT 1"); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("tx ExecContext() = %v, want ErrTxDone", err)
	}
	if _, err := tx.QueryContext(ctx, "SELECT 1"); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("tx QueryContext() = %v, want ErrTxDone", err)
	}
	if err := tx.Commit(ctx); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("second Commit() = %v, want ErrTxDone", err)
	}
	if err := tx.Rollback(ctx); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("Rollback() after Commit = %v, want ErrTxDone", err)
	}
}

func TestTxQueryRowNoRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	if _, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())

	row := tx.QueryRowContext(ctx, "SELECT id FROM t WHERE id = ?", 999)
	var id int
	if err := row.Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Scan() = %v, want sql.ErrNoRows", err)
	}
}

func TestBeginRespectsContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	db := openTestDB(t)
	if _, err := db.Begin(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Begin() = %v, want context.Canceled", err)
	}
}

func TestBeginOnClosedDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := sqliteConfig(filepath.Join(t.TempDir(), "closed.db"))
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if _, err := db.Begin(ctx); !errors.Is(err, krios.ErrClosed) {
		t.Fatalf("Begin() = %v, want ErrClosed", err)
	}
}

func TestCommitCanceledContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	if err := tx.Commit(cancelCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Commit() = %v, want context.Canceled", err)
	}
	// Transaction not marked done: a subsequent Commit with valid context should work.
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("second Commit() with valid ctx = %v, want nil", err)
	}
}

func TestRowsCloseIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	if _, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT id FROM t")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("first Close() = %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("second Close() = %v, want nil (idempotent)", err)
	}
}

func TestDBExecAfterClose(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := sqliteConfig(filepath.Join(t.TempDir(), "closed2.db"))
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if _, err := db.ExecContext(ctx, "SELECT 1"); !errors.Is(err, krios.ErrClosed) {
		t.Fatalf("ExecContext() = %v, want ErrClosed", err)
	}
}
