package access

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/testutil"
)

type readShortcutUser struct {
	ID   int64 `krios:"id,pk"`
	Name string
}

func (readShortcutUser) TableName() string { return "read_shortcut_users" }

// TestReadShortcutsCloseOnInvalidDestination: A05 regression. The public read
// shortcuts acquire a native iterator and delegate mapping to Rows.All. When
// the destination is rejected, that resource must still be released on the way
// out, exercising Select.All, Select.One, Select.Paginate, Update.All and
// Delete.All end to end through a simulated driver.
func TestReadShortcutsCloseOnInvalidDestination(t *testing.T) {
	ent, err := entity.EntityOf[readShortcutUser]()
	if err != nil {
		t.Fatal(err)
	}

	newRows := func(d *testutil.Driver) *lastRows {
		var last *testutil.Rows
		d.QueryContext = func(context.Context) (driver.Rows, error) {
			last = &testutil.Rows{Values: []driver.Value{int64(1)}}

			return last, nil
		}

		return &lastRows{last: &last}
	}

	t.Run("select all", func(t *testing.T) {
		d := &testutil.Driver{}
		native := newRows(d)
		db := openDriver(t, d)

		if err := db.Select(ent).All(context.Background(), 0); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Select.All() = %v, want errs.ErrInvalidEntity", err)
		}
		if got := native.Closes(); got != 1 {
			t.Errorf("closes = %d, want 1 (Select.All leaked on invalid destination)", got)
		}
	})

	t.Run("select one", func(t *testing.T) {
		d := &testutil.Driver{}
		native := newRows(d)
		db := openDriver(t, d)

		var dest *readShortcutUser
		if err := db.Select(ent).One(context.Background(), dest); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Select.One() = %v, want errs.ErrInvalidEntity", err)
		}
		if got := native.Closes(); got != 1 {
			t.Errorf("closes = %d, want 1 (Select.One leaked on invalid destination)", got)
		}
	})

	t.Run("select paginate", func(t *testing.T) {
		d := &testutil.Driver{}
		native := newRows(d)
		db := openDriver(t, d)

		if _, err := db.Select(ent).Paginate(context.Background(), 10, 0, 0); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Select.Paginate() = %v, want errs.ErrInvalidEntity", err)
		}
		if got := native.Closes(); got != 1 {
			t.Errorf("closes = %d, want 1 (Select.Paginate leaked on invalid destination)", got)
		}
	})

	t.Run("update all", func(t *testing.T) {
		d := &testutil.Driver{}
		native := newRows(d)
		db := openDriver(t, d)

		if err := db.Update(ent).SetValues(readShortcutUser{Name: "x"}).Where(ent.Col("id").Eq(1)).Returning().All(context.Background(), 0); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Update.All() = %v, want errs.ErrInvalidEntity", err)
		}
		if got := native.Closes(); got != 1 {
			t.Errorf("closes = %d, want 1 (Update.All leaked on invalid destination)", got)
		}
	})

	t.Run("delete all", func(t *testing.T) {
		d := &testutil.Driver{}
		native := newRows(d)
		db := openDriver(t, d)

		if err := db.Delete(ent).Where(ent.Col("id").Eq(1)).Returning().All(context.Background(), 0); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Delete.All() = %v, want errs.ErrInvalidEntity", err)
		}
		if got := native.Closes(); got != 1 {
			t.Errorf("closes = %d, want 1 (Delete.All leaked on invalid destination)", got)
		}
	})

	t.Run("tx invalid destination is not recorded", func(t *testing.T) {
		d := &testutil.Driver{}
		native := newRows(d)
		db := openDriver(t, d)

		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if err := tx.Select(ent).All(context.Background(), 0); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Tx.Select.All() = %v, want errs.ErrInvalidEntity", err)
		}
		if got := native.Closes(); got != 1 {
			t.Errorf("closes = %d, want 1 (Tx.Select.All leaked on invalid destination)", got)
		}

		// A rejected destination is a read-mapping failure observed after the
		// query started on the Tx: it is recorded and blocks Commit (D04 §5).
		if err := tx.Commit(context.Background()); err == nil {
			t.Fatal("Commit() = nil, want blocked commit after mapping failure")
		}
		if d.Commits.Load() != 0 || d.Rollbacks.Load() != 1 {
			t.Errorf("commits=%d rollbacks=%d, want 0/1", d.Commits.Load(), d.Rollbacks.Load())
		}
	})
}

// lastRows reports the close count of the most recently produced native result
// set, so subtests with several queries (Count before the page) observe the
// iterator that the failing shortcut actually acquired.
type lastRows struct {
	last **testutil.Rows
}

func (l *lastRows) Closes() int32 {
	if l.last == nil || *l.last == nil {
		return 0
	}

	return (*l.last).Closes.Load()
}
