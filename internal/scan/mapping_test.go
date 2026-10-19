package scan_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/testutil"
)

type mapUser struct {
	ID   int64
	Name string
	Bio  *string
}

type mapRequired struct {
	ID   int64
	Name string
	Bio  string
}

// tableRows is a multi-column driver.Rows used by the mapping tests.
type tableRows struct {
	columns           []string
	values            [][]driver.Value
	nextErr, closeErr error
	index             int
	closes            atomic.Int32
}

func (r *tableRows) Columns() []string { return r.columns }
func (r *tableRows) Close() error {
	r.closes.Add(1)
	return r.closeErr
}
func (r *tableRows) Next(dest []driver.Value) error {
	if r.index < len(r.values) {
		copy(dest, r.values[r.index])
		r.index++
		return nil
	}
	if r.nextErr != nil {
		return r.nextErr
	}
	return io.EOF
}

func openMapped(t *testing.T, native driver.Rows) *access.DB {
	t.Helper()
	return openDriver(t, &testutil.Driver{Query: func() (driver.Rows, error) { return native, nil }})
}

func TestRowScanStructFillsFields(t *testing.T) {
	native := &tableRows{
		columns: []string{"id", "name", "bio"},
		values:  [][]driver.Value{{int64(7), "alice", "hello"}},
	}
	db := openMapped(t, native)

	var user mapUser
	if err := db.QueryRowContext(context.Background(), "read").Scan(&user); err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if user.ID != 7 || user.Name != "alice" || user.Bio == nil || *user.Bio != "hello" {
		t.Errorf("mapped struct = %+v", user)
	}
}

func TestRowScanStructPreservesErrNoRows(t *testing.T) {
	db := openMapped(t, &tableRows{columns: []string{"id", "name", "bio"}})

	var user mapUser
	if err := db.QueryRowContext(context.Background(), "read").Scan(&user); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Scan() = %v, want sql.ErrNoRows", err)
	}
}

func TestRowScanStructNullAndConversion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		columns []string
		values  [][]driver.Value
		scan    func(scan.Row) error
		wantErr bool
		check   func(*testing.T)
	}{
		{
			name:    "null into pointer",
			columns: []string{"id", "name", "bio"},
			values:  [][]driver.Value{{int64(1), "alice", nil}},
			scan: func(row scan.Row) error {
				var user mapUser
				return row.Scan(&user)
			},
			check: func(*testing.T) {},
		},
		{
			name:    "null into non-nullable",
			columns: []string{"id", "name", "bio"},
			values:  [][]driver.Value{{int64(1), "alice", nil}},
			scan: func(row scan.Row) error {
				var required mapRequired
				return row.Scan(&required)
			},
			wantErr: true,
		},
		{
			name:    "incompatible type",
			columns: []string{"id", "name", "bio"},
			values:  [][]driver.Value{{"not an integer", "alice", "bio"}},
			scan: func(row scan.Row) error {
				var user mapUser
				return row.Scan(&user)
			},
			wantErr: true,
		},
		{
			name:    "column count mismatch",
			columns: []string{"value"},
			values:  [][]driver.Value{{int64(1)}},
			scan: func(row scan.Row) error {
				var user mapUser
				return row.Scan(&user)
			},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openMapped(t, &tableRows{columns: tc.columns, values: tc.values})
			err := tc.scan(db.QueryRowContext(context.Background(), "read"))
			if tc.wantErr {
				if err == nil || errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("Scan() = %v, want a mapping error", err)
				}
				if !strings.Contains(err.Error(), "map ") {
					t.Errorf("Scan() = %v, want table context", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Scan() = %v", err)
			}
		})
	}
}

func TestRowScanDestinationClassification(t *testing.T) {
	t.Run("nil struct pointer", func(t *testing.T) {
		db := openMapped(t, &tableRows{columns: []string{"id"}})
		var user *mapUser
		if err := db.QueryRowContext(context.Background(), "read").Scan(user); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Scan() = %v, want errs.ErrInvalidEntity", err)
		}
	})

	t.Run("unnamed struct is not an entity", func(t *testing.T) {
		db := openMapped(t, &tableRows{columns: []string{"value"}, values: [][]driver.Value{{int64(1)}}})
		var dest struct{ Value int }
		if err := db.QueryRowContext(context.Background(), "read").Scan(&dest); !errors.Is(err, errs.ErrInvalidEntity) {
			t.Fatalf("Scan() = %v, want errs.ErrInvalidEntity", err)
		}
	})

	t.Run("positional destinations keep working", func(t *testing.T) {
		db := openMapped(t, &tableRows{columns: []string{"value"}, values: [][]driver.Value{{int64(1)}}})
		var value int
		if err := db.QueryRowContext(context.Background(), "read").Scan(&value); err != nil {
			t.Fatalf("Scan() = %v", err)
		}
		if value != 1 {
			t.Errorf("value = %d, want 1", value)
		}
	})

	t.Run("scanner types stay positional", func(t *testing.T) {
		db := openMapped(t, &tableRows{
			columns: []string{"name"},
			values:  [][]driver.Value{{"alice"}},
		})
		var name sql.NullString
		if err := db.QueryRowContext(context.Background(), "read").Scan(&name); err != nil {
			t.Fatalf("Scan() = %v", err)
		}
		if !name.Valid || name.String != "alice" {
			t.Errorf("name = %+v", name)
		}
	})

	t.Run("value types without exported fields stay positional", func(t *testing.T) {
		created := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
		db := openMapped(t, &tableRows{
			columns: []string{"created"},
			values:  [][]driver.Value{{created}},
		})
		var got time.Time
		if err := db.QueryRowContext(context.Background(), "read").Scan(&got); err != nil {
			t.Fatalf("Scan() = %v", err)
		}
		if !got.Equal(created) {
			t.Errorf("created = %v, want %v", got, created)
		}
	})
}

func TestRowsAllFillsSlice(t *testing.T) {
	native := &tableRows{
		columns: []string{"id", "name", "bio"},
		values: [][]driver.Value{
			{int64(1), "alice", "a"},
			{int64(2), "bob", nil},
		},
	}
	db := openMapped(t, native)

	rows, err := db.QueryContext(context.Background(), "read")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}

	users := []mapUser{{ID: 99, Name: "sentinel"}}
	if err := rows.All(&users); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}
	if users[0].ID != 1 || users[0].Name != "alice" || users[0].Bio == nil || *users[0].Bio != "a" {
		t.Errorf("users[0] = %+v", users[0])
	}
	if users[1].ID != 2 || users[1].Name != "bob" || users[1].Bio != nil {
		t.Errorf("users[1] = %+v", users[1])
	}
	if native.closes.Load() != 1 {
		t.Errorf("closes = %d, want 1", native.closes.Load())
	}
}

func TestRowsAllMatchesColumnsByName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		columns []string
		values  [][]driver.Value
		want    mapUser
	}{
		{
			name:    "case insensitive and unknown column",
			columns: []string{"NAME", "id", "nickname"},
			values:  [][]driver.Value{{"alice", int64(1), "ignored"}},
			want:    mapUser{ID: 1, Name: "alice"},
		},
		{
			name:    "subset leaves other fields zero",
			columns: []string{"name"},
			values:  [][]driver.Value{{"alice"}},
			want:    mapUser{Name: "alice"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openMapped(t, &tableRows{columns: tc.columns, values: tc.values})
			rows, err := db.QueryContext(context.Background(), "read")
			if err != nil {
				t.Fatalf("QueryContext() = %v", err)
			}
			var users []mapUser
			if err := rows.All(&users); err != nil {
				t.Fatalf("All() = %v", err)
			}
			if len(users) != 1 {
				t.Fatalf("len(users) = %d, want 1", len(users))
			}
			if users[0] != tc.want {
				t.Errorf("users[0] = %+v, want %+v", users[0], tc.want)
			}
		})
	}
}

func TestRowsAllRejectsInvalidDestinations(t *testing.T) {
	for _, tc := range []struct {
		name string
		dest any
	}{
		{"not a pointer", []mapUser{}},
		{"nil pointer", nil},
		{"pointer to struct", &mapUser{}},
		{"slice of non-struct", new([]int)},
		{"slice of pointers", new([]*mapUser)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openMapped(t, &tableRows{columns: []string{"id"}})
			rows, err := db.QueryContext(context.Background(), "read")
			if err != nil {
				t.Fatalf("QueryContext() = %v", err)
			}
			defer rows.Close()
			if err := rows.All(tc.dest); !errors.Is(err, errs.ErrInvalidEntity) {
				t.Fatalf("All() = %v, want errs.ErrInvalidEntity", err)
			}
		})
	}
}

func TestRowsAllFailureKeepsDestinationAndCloses(t *testing.T) {
	native := &tableRows{
		columns: []string{"id", "name", "bio"},
		values: [][]driver.Value{
			{int64(1), "alice", "a"},
			{"not an integer", "bob", "b"},
		},
	}
	db := openMapped(t, native)

	rows, err := db.QueryContext(context.Background(), "read")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}

	users := []mapUser{{ID: 99, Name: "sentinel"}}
	if err := rows.All(&users); err == nil {
		t.Fatal("All() = nil, want a conversion error")
	}
	if len(users) != 1 || users[0].Name != "sentinel" {
		t.Errorf("destination changed: %+v", users)
	}
	if native.closes.Load() != 1 {
		t.Errorf("closes = %d, want 1", native.closes.Load())
	}
}

func TestRowsAllEmptyResultReplacesDestination(t *testing.T) {
	db := openMapped(t, &tableRows{columns: []string{"id"}})

	rows, err := db.QueryContext(context.Background(), "read")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}

	users := []mapUser{{ID: 99, Name: "sentinel"}}
	if err := rows.All(&users); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(users) != 0 {
		t.Errorf("users = %+v, want empty slice", users)
	}
}

func TestRowsAllPropagatesIterationError(t *testing.T) {
	cause := &testutil.ReadFailure{}
	db := openMapped(t, &tableRows{columns: []string{"id"}, nextErr: cause})

	rows, err := db.QueryContext(context.Background(), "read")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}

	var users []mapUser
	if err := rows.All(&users); !errors.Is(err, cause) {
		t.Fatalf("All() = %v, want %v", err, cause)
	}
}

func TestRowsColumnsReportsResultset(t *testing.T) {
	db := openMapped(t, &tableRows{columns: []string{"id", "name"}})

	rows, err := db.QueryContext(context.Background(), "read")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	columns, err := rows.Columns()
	if err != nil {
		t.Fatalf("Columns() = %v", err)
	}
	if !equalStrings(columns, []string{"id", "name"}) {
		t.Errorf("Columns() = %v", columns)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if _, err := rows.Columns(); err == nil {
		t.Error("Columns() after Close = nil, want error")
	}
}

func TestRowScanStructIsPositionalByDeclarationOrder(t *testing.T) {
	// Row.Scan(&T) ignores column names and fills fields in declaration order.
	// Result columns are labeled differently, yet values land positionally:
	// column 0 -> ID, column 1 -> Name, column 2 -> Bio.
	native := &tableRows{
		columns: []string{"first", "second", "third"},
		values:  [][]driver.Value{{int64(7), "alice", "hello"}},
	}
	db := openMapped(t, native)

	var user mapUser
	if err := db.QueryRowContext(context.Background(), "read").Scan(&user); err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if user.ID != 7 || user.Name != "alice" || user.Bio == nil || *user.Bio != "hello" {
		t.Errorf("positional mapping = %+v, want fields filled by result position", user)
	}
}

func TestRowsAllKeepsRecordsIndependent(t *testing.T) {
	native := &tableRows{
		columns: []string{"id", "name", "bio"},
		values: [][]driver.Value{
			{int64(1), "alice", "a"},
			{int64(2), "bob", "b"},
			{int64(3), "carol", nil},
		},
	}
	db := openMapped(t, native)

	rows, err := db.QueryContext(context.Background(), "read")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}

	var users []mapUser
	if err := rows.All(&users); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("len(users) = %d, want 3", len(users))
	}
	for i, want := range []mapUser{{ID: 1, Name: "alice"}, {ID: 2, Name: "bob"}, {ID: 3, Name: "carol"}} {
		if users[i].ID != want.ID || users[i].Name != want.Name {
			t.Errorf("users[%d] = %+v, want id=%d name=%s", i, users[i], want.ID, want.Name)
		}
	}
	if users[0].Bio == nil || *users[0].Bio != "a" || users[1].Bio == nil || *users[1].Bio != "b" || users[2].Bio != nil {
		t.Errorf("bio values leaked between records: %+v", users)
	}
}

func TestMappedReadFailuresBlockCommit(t *testing.T) {
	t.Run("all iteration failure", func(t *testing.T) {
		d := &testutil.Driver{Query: func() (driver.Rows, error) {
			return &tableRows{columns: []string{"id"}, nextErr: &testutil.ReadFailure{}}, nil
		}}
		tx := beginDriver(t, openDriver(t, d), context.Background())

		rows, err := tx.QueryContext(context.Background(), "read")
		if err != nil {
			t.Fatalf("QueryContext() = %v", err)
		}
		var users []mapUser
		allErr := rows.All(&users)
		if allErr == nil {
			t.Fatal("All() = nil, want read failure")
		}
		var typed *testutil.ReadFailure
		if !errors.As(allErr, &typed) {
			t.Fatalf("All() = %v, want read failure", allErr)
		}

		commitErr := tx.Commit(context.Background())
		if !errors.As(commitErr, &typed) {
			t.Fatalf("Commit = %v, want blocked commit preserving the read failure", commitErr)
		}
		if d.Commits.Load() != 0 || d.Rollbacks.Load() != 1 {
			t.Fatal("wrong native finalization")
		}
		assertTxDone(t, tx)
	})

	t.Run("row conversion failure", func(t *testing.T) {
		d := &testutil.Driver{Query: func() (driver.Rows, error) {
			return &tableRows{
				columns: []string{"id", "name", "bio"},
				values:  [][]driver.Value{{"not an integer", "alice", "bio"}},
			}, nil
		}}
		tx := beginDriver(t, openDriver(t, d), context.Background())

		var user mapUser
		err := tx.QueryRowContext(context.Background(), "read").Scan(&user)
		if err == nil || errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("Scan() = %v, want a mapping error", err)
		}
		if err := tx.Commit(context.Background()); err == nil {
			t.Fatal("Commit = nil, want blocked commit")
		}
		if d.Commits.Load() != 0 || d.Rollbacks.Load() != 1 {
			t.Fatal("wrong native finalization")
		}
		assertTxDone(t, tx)
	})
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
