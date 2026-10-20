package tests

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

type sqlfileAccount struct {
	ID   int64   `krios:"id,pk"`
	Name string  `krios:"name"`
	Bio  *string `krios:"bio"`
}

func (sqlfileAccount) TableName() string { return "sqlfile_accounts" }

func sqlfileFS() fstest.MapFS {
	return fstest.MapFS{
		"create.sql": {Data: []byte("CREATE TABLE sqlfile_accounts (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)")},
		"insert.sql": {Data: []byte("INSERT INTO sqlfile_accounts (id, name, bio) VALUES (?, ?, ?)")},
		"by_id.sql":  {Data: []byte("SELECT id, name, bio FROM sqlfile_accounts WHERE id = ?")},
		"all.sql":    {Data: []byte("SELECT id, name, bio FROM sqlfile_accounts ORDER BY id")},
		"names.sql":  {Data: []byte("SELECT name FROM sqlfile_accounts ORDER BY id")},
		"blank.sql":  {Data: []byte("   \n ")},
	}
}

func seedSQLFileDB(t *testing.T) (*krios.DB, fstest.MapFS) {
	t.Helper()

	db := openTestDB(t)
	fsys := sqlfileFS()

	create, err := krios.NewSQLFile(fsys, "create.sql")
	if err != nil {
		t.Fatalf("NewSQLFile(create) = %v", err)
	}

	if _, err := db.File(create).Exec(context.Background()); err != nil {
		t.Fatalf("create schema = %v", err)
	}

	return db, fsys
}

func TestSQLFileExecAndQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, fsys := seedSQLFileDB(t)

	insert, err := krios.NewSQLFile(fsys, "insert.sql")
	if err != nil {
		t.Fatalf("NewSQLFile(insert) = %v", err)
	}

	res, err := db.File(insert).Exec(ctx, 1, "alice", "reader")
	if err != nil {
		t.Fatalf("insert = %v", err)
	}

	if n, ok := res.RowsAffected(); !ok || n != 1 {
		t.Errorf("RowsAffected() = %d, %v; want 1, true", n, ok)
	}

	byID, err := krios.NewSQLFile(fsys, "by_id.sql")
	if err != nil {
		t.Fatalf("NewSQLFile(by_id) = %v", err)
	}

	var got sqlfileAccount
	if err := db.File(byID).One(ctx, &got, 1); err != nil {
		t.Fatalf("One = %v", err)
	}

	if got.ID != 1 || got.Name != "alice" || got.Bio == nil || *got.Bio != "reader" {
		t.Errorf("One() = %+v, want id 1, name alice, bio reader", got)
	}
}

func TestSQLFileQueryRowNoRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, fsys := seedSQLFileDB(t)

	byID, err := krios.NewSQLFile(fsys, "by_id.sql")
	if err != nil {
		t.Fatalf("NewSQLFile(by_id) = %v", err)
	}

	var got sqlfileAccount
	if err := db.File(byID).QueryRow(ctx, 99).Scan(&got); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("QueryRow().Scan() = %v, want sql.ErrNoRows", err)
	}
}

func TestSQLFileAllMapsRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, fsys := seedSQLFileDB(t)

	insert, err := krios.NewSQLFile(fsys, "insert.sql")
	if err != nil {
		t.Fatalf("NewSQLFile(insert) = %v", err)
	}
	if _, err := db.File(insert).Exec(ctx, 1, "alice", "reader"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.File(insert).Exec(ctx, 2, "bob", nil); err != nil {
		t.Fatal(err)
	}

	all, err := krios.NewSQLFile(fsys, "all.sql")
	if err != nil {
		t.Fatalf("NewSQLFile(all) = %v", err)
	}

	var accounts []sqlfileAccount
	if err := db.File(all).All(ctx, &accounts); err != nil {
		t.Fatalf("All = %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("All() len = %d, want 2", len(accounts))
	}
	if accounts[0].Name != "alice" || accounts[1].Name != "bob" {
		t.Errorf("All() = %+v", accounts)
	}
	if accounts[1].Bio != nil {
		t.Errorf("bob bio = %v, want nil", accounts[1].Bio)
	}
}

func TestSQLFileQueryIteration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, fsys := seedSQLFileDB(t)

	insert, err := krios.NewSQLFile(fsys, "insert.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.File(insert).Exec(ctx, 1, "alice", nil); err != nil {
		t.Fatal(err)
	}

	all, err := krios.NewSQLFile(fsys, "names.sql")
	if err != nil {
		t.Fatal(err)
	}

	rows, err := db.File(all).Query(ctx)
	if err != nil {
		t.Fatalf("Query = %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Scan = %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Err = %v", err)
	}
	if len(names) != 1 || names[0] != "alice" {
		t.Errorf("names = %v", names)
	}
}

func TestSQLFileTransactionCommitAndRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, fsys := seedSQLFileDB(t)

	insert, err := krios.NewSQLFile(fsys, "insert.sql")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}

	if _, err := tx.File(insert).Exec(ctx, 1, "alice", nil); err != nil {
		t.Fatalf("tx insert = %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit = %v", err)
	}

	if n := countRows(t, db, "sqlfile_accounts"); n != 1 {
		t.Fatalf("rows after commit = %d, want 1", n)
	}

	tx2, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	if _, err := tx2.File(insert).Exec(ctx, 2, "bob", nil); err != nil {
		t.Fatalf("tx2 insert = %v", err)
	}
	if err := tx2.Rollback(ctx); err != nil {
		t.Fatalf("Rollback = %v", err)
	}

	if n := countRows(t, db, "sqlfile_accounts"); n != 1 {
		t.Fatalf("rows after rollback = %d, want 1", n)
	}
}

func TestSQLFileGuards(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, fsys := seedSQLFileDB(t)

	all, err := krios.NewSQLFile(fsys, "all.sql")
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := db.File(all).Exec(ctx); !errors.Is(err, krios.ErrClosed) {
		t.Fatalf("Exec() after Close = %v, want ErrClosed", err)
	}

	if err := db.File(all).All(ctx, &[]sqlfileAccount{}); !errors.Is(err, krios.ErrClosed) {
		t.Fatalf("All() after Close = %v, want ErrClosed", err)
	}
}

func TestSQLFileTxDone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, fsys := seedSQLFileDB(t)

	all, err := krios.NewSQLFile(fsys, "all.sql")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.File(all).Exec(ctx); !errors.Is(err, krios.ErrTxDone) {
		t.Fatalf("Exec() after finalize = %v, want ErrTxDone", err)
	}
}

func TestSQLFileNilRunner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	if err := db.File(nil).QueryRow(ctx).Scan(new(int)); !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("File(nil).QueryRow().Scan() = %v, want ErrInvalidQuery", err)
	}
}

func TestNewSQLFileErrorsIntegration(t *testing.T) {
	t.Parallel()

	fsys := sqlfileFS()

	if _, err := krios.NewSQLFile(fsys, "missing.sql"); !errors.Is(err, krios.ErrFileNotFound) {
		t.Fatalf("NewSQLFile(missing) = %v, want ErrFileNotFound", err)
	}
	if _, err := krios.NewSQLFile(fsys, "blank.sql"); !errors.Is(err, krios.ErrFileNotFound) {
		t.Fatalf("NewSQLFile(blank) = %v, want ErrFileNotFound", err)
	}
}
