package sqlfile_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/sqlfile"
	"github.com/mineot/krios/internal/testutil"
)

func fsFixture() fstest.MapFS {
	return fstest.MapFS{
		"queries/list.sql":   {Data: []byte("SELECT id, name FROM users")},
		"queries/by_id.sql":  {Data: []byte("SELECT id, name FROM users WHERE id = $1")},
		"queries/blank.sql":  {Data: []byte("   \n\t ")},
		"queries/dir":        {Mode: fs.ModeDir},
		"queries/spaces.sql": {Data: []byte("  SELECT 1  ")},
	}
}

func mustFile(t *testing.T, fsys fs.FS, path string) *sqlfile.SQLFile {
	t.Helper()

	f, err := sqlfile.NewSQLFile(fsys, path)
	if err != nil {
		t.Fatalf("NewSQLFile(%q) = %v", path, err)
	}

	return f
}

func openDriver(t *testing.T, d *testutil.Driver) *access.DB {
	t.Helper()

	db, err := access.Open(context.Background(), testutil.Config(d))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})

	return db
}

func TestNewSQLFileLoadsResource(t *testing.T) {
	f := mustFile(t, fsFixture(), "queries/list.sql")

	if f.Name() != "queries/list.sql" {
		t.Errorf("Name() = %q", f.Name())
	}

	if want := "SELECT id, name FROM users"; f.Text() != want {
		t.Errorf("Text() = %q, want %q", f.Text(), want)
	}
}

func TestNewSQLFilePreservesText(t *testing.T) {
	f := mustFile(t, fsFixture(), "queries/spaces.sql")

	if want := "  SELECT 1  "; f.Text() != want {
		t.Errorf("Text() = %q, want %q", f.Text(), want)
	}
}

func TestNewSQLFileErrors(t *testing.T) {
	cases := []struct {
		name string
		fsys fs.FS
		path string
	}{
		{name: "nil fs", fsys: nil, path: "queries/list.sql"},
		{name: "invalid path", fsys: fsFixture(), path: "../escape.sql"},
		{name: "missing", fsys: fsFixture(), path: "queries/missing.sql"},
		{name: "directory", fsys: fsFixture(), path: "queries/dir"},
		{name: "blank", fsys: fsFixture(), path: "queries/blank.sql"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := sqlfile.NewSQLFile(tc.fsys, tc.path)

			if f != nil {
				t.Errorf("NewSQLFile() returned %v on error", f)
			}

			if !errors.Is(err, errs.ErrFileNotFound) {
				t.Fatalf("NewSQLFile() = %v, want errs.ErrFileNotFound", err)
			}
		})
	}
}

func TestNewSQLFileMissingWrapsNotExist(t *testing.T) {
	_, err := sqlfile.NewSQLFile(fsFixture(), "queries/missing.sql")

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("NewSQLFile() = %v, want fs.ErrNotExist", err)
	}
}

func TestNilFileIsRemembered(t *testing.T) {
	d := &testutil.Driver{}
	db := openDriver(t, d)

	// A nil runner reports the construction failure before any executor guard.
	if _, err := db.File(nil).Exec(context.Background()); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatalf("File(nil).Exec() = %v, want errs.ErrInvalidQuery", err)
	}

	// The remembered failure wins even when the access is closed.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := db.File(nil).QueryRow(context.Background()).Scan(new(int)); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatalf("File(nil).QueryRow().Scan() = %v, want errs.ErrInvalidQuery", err)
	}
}

func TestFilePassesSQLAndArgsIntact(t *testing.T) {
	d := &testutil.Driver{Exec: func(context.Context) error { return nil }}
	db := openDriver(t, d)

	text := "INSERT INTO users (name, bio) VALUES ($1, $2)"
	f := mustFile(t, fstest.MapFS{"q.sql": {Data: []byte(text)}}, "q.sql")

	if _, err := db.File(f).Exec(context.Background(), "ana", nil); err != nil {
		t.Fatalf("Exec() = %v", err)
	}

	if got := d.LastQuery(); got != text {
		t.Errorf("driver query = %q, want %q", got, text)
	}

	args := d.LastArgs()
	if len(args) != 2 || args[0].Value != "ana" || args[1].Value != nil {
		t.Errorf("driver args = %v, want [ana <nil>]", args)
	}
}

func TestFileQuerySendsText(t *testing.T) {
	d := &testutil.Driver{Query: func() (driver.Rows, error) { return &testutil.Rows{}, nil }}
	db := openDriver(t, d)

	text := "SELECT id, name FROM users WHERE id = $1"
	f := mustFile(t, fstest.MapFS{"q.sql": {Data: []byte(text)}}, "q.sql")

	rows, err := db.File(f).Query(context.Background(), 7)
	if err != nil {
		t.Fatalf("Query() = %v", err)
	}

	if err := rows.Close(); err != nil {
		t.Fatalf("rows.Close() = %v", err)
	}

	if got := d.LastQuery(); got != text {
		t.Errorf("driver query = %q, want %q", got, text)
	}
}

func TestFileExecAndGuards(t *testing.T) {
	f := mustFile(t, fsFixture(), "queries/by_id.sql")

	d := &testutil.Driver{}
	db := openDriver(t, d)

	res, err := db.File(f).Exec(context.Background(), 1)
	if err != nil {
		t.Fatalf("Exec() = %v", err)
	}

	if n, ok := res.RowsAffected(); !ok || n != 1 {
		t.Errorf("RowsAffected() = %d, %v", n, ok)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := db.File(f).Exec(context.Background(), 1); !errors.Is(err, errs.ErrClosed) {
		t.Fatalf("Exec() after Close = %v, want errs.ErrClosed", err)
	}
}

func TestFileQueryRowNoRowsPreserved(t *testing.T) {
	f := mustFile(t, fsFixture(), "queries/by_id.sql")

	d := &testutil.Driver{
		QueryContext: func(context.Context) (driver.Rows, error) {
			return &testutil.Rows{NextErr: sql.ErrNoRows}, nil
		},
	}
	db := openDriver(t, d)

	var value int
	if err := db.File(f).QueryRow(context.Background(), 99).Scan(&value); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("QueryRow().Scan() = %v, want sql.ErrNoRows", err)
	}
}

func TestFileOneAndAllRejectInvalidDest(t *testing.T) {
	f := mustFile(t, fsFixture(), "queries/list.sql")

	native := &testutil.Rows{Values: []driver.Value{int64(1)}}
	d := &testutil.Driver{Query: func() (driver.Rows, error) {
		return native, nil
	}}
	db := openDriver(t, d)

	if err := db.File(f).One(context.Background(), 0); err == nil {
		t.Fatal("One(non-pointer) = nil, want an error")
	}

	if err := db.File(f).All(context.Background(), 0); !errors.Is(err, errs.ErrInvalidEntity) {
		t.Fatalf("All(non-pointer) = %v, want errs.ErrInvalidEntity", err)
	}

	if got := native.Closes.Load(); got != 2 {
		t.Errorf("native rows closed %d times, want 2 (resource leaked on invalid destination)", got)
	}
}

func TestFileLateFailureBlocksCommit(t *testing.T) {
	f := mustFile(t, fsFixture(), "queries/list.sql")

	cause := &testutil.ReadFailure{}
	d := &testutil.Driver{
		QueryContext: func(context.Context) (driver.Rows, error) {
			return &testutil.Rows{NextErr: cause}, nil
		},
	}
	db := openDriver(t, d)

	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	rows, err := tx.File(f).Query(context.Background())
	if err != nil {
		t.Fatalf("Query() = %v", err)
	}

	for rows.Next() {
	}

	if err := rows.Err(); !errors.Is(err, cause) {
		t.Fatalf("rows.Err() = %v, want cause", err)
	}

	if err := rows.Close(); err != nil {
		t.Fatalf("rows.Close() = %v", err)
	}

	if err := tx.Commit(context.Background()); !errors.Is(err, cause) {
		t.Fatalf("Commit() = %v, want blocked by cause", err)
	}

	if d.Rollbacks.Load() != 1 {
		t.Errorf("Rollbacks = %d, want 1 (auto-rollback)", d.Rollbacks.Load())
	}
}

func TestFileTxDone(t *testing.T) {
	f := mustFile(t, fsFixture(), "queries/list.sql")

	d := &testutil.Driver{}
	db := openDriver(t, d)

	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.File(f).Exec(context.Background()); !errors.Is(err, errs.ErrTxDone) {
		t.Fatalf("Exec() after finalize = %v, want errs.ErrTxDone", err)
	}
}
