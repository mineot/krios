package access

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/testutil"
)

type writeShortcutUser struct {
	ID   int64 `krios:"id,pk"`
	Name string
}

func (writeShortcutUser) TableName() string { return "write_shortcut_users" }

// TestWriteShortcutsDoNotStartOnBuildError: C04-P03. Construction failures
// (missing values/where/returning, unknown column) must be reported before the
// executor is reached, so no statement starts on the driver. The counters prove
// it even though the builder owns the executor.
func TestWriteShortcutsDoNotStartOnBuildError(t *testing.T) {
	ent, err := entity.EntityOf[writeShortcutUser]()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("insert without values", func(t *testing.T) {
		d := &testutil.Driver{}
		db := openDriver(t, d)

		if _, err := db.Insert(ent).Exec(context.Background()); !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("Insert.Exec() = %v, want errs.ErrInvalidQuery", err)
		}
		assertNoStatement(t, d)
	})

	t.Run("update without where", func(t *testing.T) {
		d := &testutil.Driver{}
		db := openDriver(t, d)

		if _, err := db.Update(ent).SetValues(writeShortcutUser{ID: 1, Name: "x"}).Exec(context.Background()); !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("Update.Exec() = %v, want errs.ErrInvalidQuery", err)
		}
		assertNoStatement(t, d)
	})

	t.Run("update all without returning", func(t *testing.T) {
		d := &testutil.Driver{}
		db := openDriver(t, d)

		err := db.Update(ent).SetValues(writeShortcutUser{ID: 1, Name: "x"}).Where(ent.Col("id").Eq(1)).All(context.Background(), &[]writeShortcutUser{})
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("Update.All() = %v, want errs.ErrInvalidQuery", err)
		}
		assertNoStatement(t, d)
	})

	t.Run("delete all without returning", func(t *testing.T) {
		d := &testutil.Driver{}
		db := openDriver(t, d)

		err := db.Delete(ent).Where(ent.Col("id").Eq(1)).All(context.Background(), &[]writeShortcutUser{})
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("Delete.All() = %v, want errs.ErrInvalidQuery", err)
		}
		assertNoStatement(t, d)
	})

	t.Run("select unknown column", func(t *testing.T) {
		d := &testutil.Driver{}
		db := openDriver(t, d)

		err := db.Select(ent).Where(ent.Col("missing").Eq(1)).All(context.Background(), &[]writeShortcutUser{})
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("Select.All() = %v, want errs.ErrInvalidQuery", err)
		}
		assertNoStatement(t, d)
	})

	t.Run("build error does not block tx commit", func(t *testing.T) {
		d := &testutil.Driver{}
		db := openDriver(t, d)

		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if _, err := tx.Delete(ent).Exec(context.Background()); !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("Tx.Delete.Exec() = %v, want errs.ErrInvalidQuery", err)
		}
		if err := tx.Select(ent).Where(ent.Col("missing").Eq(1)).All(context.Background(), &[]writeShortcutUser{}); !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("Tx.Select.All() = %v, want errs.ErrInvalidQuery", err)
		}
		assertNoStatement(t, d)

		// A construction failure never starts a statement, so it is not
		// recorded and must not block Commit (D04 §5).
		if err := tx.Commit(context.Background()); err != nil {
			t.Fatalf("Commit() = %v, want nil after a pre-execution failure", err)
		}
		if d.Commits.Load() != 1 || d.Rollbacks.Load() != 0 {
			t.Errorf("commits=%d rollbacks=%d, want 1/0", d.Commits.Load(), d.Rollbacks.Load())
		}
	})
}

func assertNoStatement(t *testing.T, d *testutil.Driver) {
	t.Helper()

	if got := d.Execs.Load(); got != 0 {
		t.Errorf("Execs = %d, want 0 (statement started on build error)", got)
	}
	if got := d.Queries.Load(); got != 0 {
		t.Errorf("Queries = %d, want 0 (statement started on build error)", got)
	}
	if got := d.LastQuery(); got != "" {
		t.Errorf("LastQuery() = %q, want empty", got)
	}
}

// TestWriteShortcutsErrNoRows: C04-P03. sql.ErrNoRows is preserved by the
// Returning single-row shortcut (Insert.One) and never blocks Commit.
func TestWriteShortcutsErrNoRows(t *testing.T) {
	ent, err := entity.EntityOf[writeShortcutUser]()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("insert one preserves err no rows", func(t *testing.T) {
		native := &testutil.Rows{NextErr: sql.ErrNoRows}
		d := &testutil.Driver{Query: func() (driver.Rows, error) { return native, nil }}
		db := openDriver(t, d)

		var out writeShortcutUser
		err := db.Insert(ent).SetValues(writeShortcutUser{ID: 1, Name: "x"}).Returning().One(context.Background(), &out)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("Insert.One() = %v, want sql.ErrNoRows", err)
		}
		if native.Closes.Load() != 1 {
			t.Errorf("closes = %d, want 1", native.Closes.Load())
		}
	})

	t.Run("insert one success closes", func(t *testing.T) {
		native := &multiRows{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(1), "alice"}}}
		d := &testutil.Driver{Query: func() (driver.Rows, error) { return native, nil }}
		db := openDriver(t, d)

		var out writeShortcutUser
		if err := db.Insert(ent).SetValues(writeShortcutUser{ID: 1, Name: "x"}).Returning().One(context.Background(), &out); err != nil {
			t.Fatalf("Insert.One() = %v", err)
		}
		if out.ID != 1 || out.Name != "alice" {
			t.Errorf("out = %+v, want id=1 name=alice", out)
		}
		if native.closes.Load() != 1 {
			t.Errorf("closes = %d, want 1", native.closes.Load())
		}
	})

	t.Run("err no rows does not block tx commit", func(t *testing.T) {
		native := &testutil.Rows{NextErr: sql.ErrNoRows}
		d := &testutil.Driver{Query: func() (driver.Rows, error) { return native, nil }}
		db := openDriver(t, d)

		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		var out writeShortcutUser
		if err := tx.Insert(ent).SetValues(writeShortcutUser{ID: 1, Name: "x"}).Returning().One(context.Background(), &out); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("Tx.Insert.One() = %v, want sql.ErrNoRows", err)
		}

		if err := tx.Commit(context.Background()); err != nil {
			t.Fatalf("Commit() = %v, want nil (ErrNoRows is not a failure)", err)
		}
		if d.Commits.Load() != 1 || d.Rollbacks.Load() != 0 {
			t.Errorf("commits=%d rollbacks=%d, want 1/0", d.Commits.Load(), d.Rollbacks.Load())
		}
	})

	t.Run("update all empty result yields empty slice", func(t *testing.T) {
		native := &testutil.Rows{}
		d := &testutil.Driver{Query: func() (driver.Rows, error) { return native, nil }}
		db := openDriver(t, d)

		out := []writeShortcutUser{{ID: 99, Name: "sentinel"}}
		err := db.Update(ent).SetValues(writeShortcutUser{ID: 1, Name: "x"}).Where(ent.Col("id").Eq(1)).Returning().All(context.Background(), &out)
		if err != nil {
			t.Fatalf("Update.All() = %v, want nil", err)
		}
		if len(out) != 0 {
			t.Errorf("out = %+v, want empty slice", out)
		}
		if native.Closes.Load() != 1 {
			t.Errorf("closes = %d, want 1", native.Closes.Load())
		}
	})

	t.Run("delete all empty result yields empty slice", func(t *testing.T) {
		native := &testutil.Rows{}
		d := &testutil.Driver{Query: func() (driver.Rows, error) { return native, nil }}
		db := openDriver(t, d)

		out := []writeShortcutUser{{ID: 99, Name: "sentinel"}}
		err := db.Delete(ent).Where(ent.Col("id").Eq(1)).Returning().All(context.Background(), &out)
		if err != nil {
			t.Fatalf("Delete.All() = %v, want nil", err)
		}
		if len(out) != 0 {
			t.Errorf("out = %+v, want empty slice", out)
		}
		if native.Closes.Load() != 1 {
			t.Errorf("closes = %d, want 1", native.Closes.Load())
		}
	})
}

// TestWriteShortcutsLateFailureBlocksCommit: C04-P03. A mapping failure
// observed on a statement already started in a Tx (Paginate page, write
// Returning.All) is recorded and blocks Commit with an automatic rollback.
func TestWriteShortcutsLateFailureBlocksCommit(t *testing.T) {
	ent, err := entity.EntityOf[writeShortcutUser]()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("paginate page conversion", func(t *testing.T) {
		d := &testutil.Driver{}
		calls := 0
		d.QueryContext = func(context.Context) (driver.Rows, error) {
			calls++
			if calls == 1 {
				// Count query: a valid integer cell.
				return &testutil.Rows{Values: []driver.Value{int64(3)}}, nil
			}

			// Page query: an unconvertible cell for the struct column.
			return &multiRows{columns: []string{"id", "name"}, values: [][]driver.Value{{"not an integer", "alice"}}}, nil
		}
		db := openDriver(t, d)

		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		var out []writeShortcutUser
		if _, err := tx.Select(ent).Paginate(context.Background(), 10, 0, &out); err == nil {
			t.Fatal("Paginate() = nil, want a mapping error")
		}

		if err := tx.Commit(context.Background()); err == nil {
			t.Fatal("Commit() = nil, want blocked commit after a mapping failure")
		}
		if d.Commits.Load() != 0 || d.Rollbacks.Load() != 1 {
			t.Errorf("commits=%d rollbacks=%d, want 0/1", d.Commits.Load(), d.Rollbacks.Load())
		}
	})

	t.Run("update returning all conversion", func(t *testing.T) {
		d := &testutil.Driver{Query: func() (driver.Rows, error) {
			return &multiRows{columns: []string{"id", "name"}, values: [][]driver.Value{{"not an integer", "alice"}}}, nil
		}}
		db := openDriver(t, d)

		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		var out []writeShortcutUser
		if err := tx.Update(ent).SetValues(writeShortcutUser{ID: 1, Name: "x"}).Where(ent.Col("id").Eq(1)).Returning().All(context.Background(), &out); err == nil {
			t.Fatal("Update.All() = nil, want a mapping error")
		}

		if err := tx.Commit(context.Background()); err == nil {
			t.Fatal("Commit() = nil, want blocked commit after a mapping failure")
		}
		if d.Commits.Load() != 0 || d.Rollbacks.Load() != 1 {
			t.Errorf("commits=%d rollbacks=%d, want 0/1", d.Commits.Load(), d.Rollbacks.Load())
		}
	})
}

// multiRows is a multi-column driver.Rows fixture for shortcuts whose struct
// mapping matches columns by name (Rows.All) or position (Row.Scan).
type multiRows struct {
	columns []string
	values  [][]driver.Value
	index   int
	closes  atomic.Int32
}

func (r *multiRows) Columns() []string { return r.columns }

func (r *multiRows) Close() error {
	r.closes.Add(1)

	return nil
}

func (r *multiRows) Next(dest []driver.Value) error {
	if r.index < len(r.values) {
		copy(dest, r.values[r.index])
		r.index++

		return nil
	}

	return io.EOF
}
