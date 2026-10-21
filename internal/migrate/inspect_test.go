package migrate_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/migrate"
	"github.com/mineot/krios/internal/testutil"
)

// This file covers C03-P01: the object-existence inspection must bind the
// consumer-supplied name as an argument (never interpolate it), scope simple
// names to the schema of the connection, support qualified names on
// MySQL/PostgreSQL, reject them on SQLite and preserve the cause of query
// failures. Evidence is simulated with the testutil driver; real-server
// homonyms belong to E09/E10 (scenario M07).

// countRows serves a single COUNT(*) row for the object-existence SELECT.
type countRows struct {
	value int64
	done  bool
}

func (r *countRows) Columns() []string { return []string{"count"} }

func (r *countRows) Close() error { return nil }

func (r *countRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}

	dest[0] = r.value
	r.done = true

	return nil
}

// inspectDriver returns a driver whose history probe reports a missing table
// and whose existence probe reports the object with the given count. The two
// queries are distinguished by text so the behaviour is stable across the
// several probes Prepare performs.
func inspectDriver(objectCount int64) *testutil.Driver {
	d := &testutil.Driver{}

	d.QueryContext = func(context.Context) (driver.Rows, error) {
		q := d.LastQuery()

		if strings.Contains(q, "sqlite_master") || strings.Contains(q, "information_schema") {
			return &countRows{value: objectCount}, nil
		}

		return nil, errors.New("no such table: migrations")
	}

	return d
}

func inspectBaseConfig() *config.MigrationsConfig {
	target := 1

	return &config.MigrationsConfig{
		FS: fstest.MapFS{
			"m/001_users.sql":      {Data: []byte("CREATE TABLE users (id INTEGER PRIMARY KEY)")},
			"m/001_users.down.sql": {Data: []byte("DROP TABLE users")},
		},
		TargetVersion: &target,
		Versions: []config.MigrationVersion{
			{Version: 1, Up: []string{"m/001_users.sql"}, Down: []string{"m/001_users.down.sql"}},
		},
		ManagedObjects: []string{"users"},
	}
}

func openInspectDriver(t *testing.T, d *testutil.Driver) *access.DB {
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

func prepareInspect(t *testing.T, db *access.DB, cfg *config.MigrationsConfig, typ config.DBType) (*migrate.Plan, error) {
	t.Helper()

	d := dialect.For(typ)
	if d == nil {
		t.Fatalf("dialect.For(%q) = nil", typ)
	}

	return migrate.Prepare(context.Background(), db, cfg, d)
}

func assertArgs(t *testing.T, d *testutil.Driver, want ...any) {
	t.Helper()

	got := d.LastArgs()
	if len(got) != len(want) {
		t.Fatalf("LastArgs() = %v, want %v", got, want)
	}

	for i, w := range want {
		if got[i].Value != w {
			t.Errorf("arg[%d] = %#v, want %#v", i, got[i].Value, w)
		}
	}
}

func TestExistenceQueryBindsNameInThreeDialects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		typ       config.DBType
		object    string
		wantQuery string
		wantArgs  []any
	}{
		{
			name:      "sqlite simple",
			typ:       config.DBTypeSQLite,
			object:    "users",
			wantQuery: `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			wantArgs:  []any{"users"},
		},
		{
			name:      "mysql simple scoped to current database",
			typ:       config.DBTypeMySQL,
			object:    "users",
			wantQuery: "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			wantArgs:  []any{"users"},
		},
		{
			name:      "postgres simple scoped to current schema",
			typ:       config.DBTypePostgres,
			object:    "users",
			wantQuery: "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1",
			wantArgs:  []any{"users"},
		},
		{
			name:      "mysql qualified",
			typ:       config.DBTypeMySQL,
			object:    "app.users",
			wantQuery: "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?",
			wantArgs:  []any{"app", "users"},
		},
		{
			name:      "postgres qualified numbered placeholders",
			typ:       config.DBTypePostgres,
			object:    "app.users",
			wantQuery: "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2",
			wantArgs:  []any{"app", "users"},
		},
		{
			name:      "sqlite name with apostrophe stays an argument",
			typ:       config.DBTypeSQLite,
			object:    "o'brien",
			wantQuery: `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			wantArgs:  []any{"o'brien"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := inspectDriver(0)
			db := openInspectDriver(t, d)

			cfg := inspectBaseConfig()
			cfg.ManagedObjects = []string{tt.object}

			_, err := prepareInspect(t, db, cfg, tt.typ)
			if err != nil {
				t.Fatalf("Prepare() = %v, want nil", err)
			}

			if got := d.LastQuery(); got != tt.wantQuery {
				t.Errorf("LastQuery() =\n%s\nwant\n%s", got, tt.wantQuery)
			}

			assertArgs(t, d, tt.wantArgs...)

			if strings.Contains(d.LastQuery(), "'"+tt.object+"'") {
				t.Errorf("query interpolates the object name: %s", d.LastQuery())
			}
		})
	}
}

func TestExistenceConfigSetsManagedObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		typ    config.DBType
		object string
	}{
		{config.DBTypeMySQL, "users"},
		{config.DBTypePostgres, "users"},
		{config.DBTypeSQLite, "users"},
	}

	for _, tt := range tests {
		t.Run(string(tt.typ)+"/"+tt.object, func(t *testing.T) {
			t.Parallel()

			d := inspectDriver(1)
			db := openInspectDriver(t, d)

			cfg := inspectBaseConfig()
			cfg.ManagedObjects = []string{tt.object}

			_, err := prepareInspect(t, db, cfg, tt.typ)
			if !errors.Is(err, errs.ErrInvalidQuery) {
				t.Fatalf("Prepare() = %v, want ErrInvalidQuery", err)
			}
		})
	}
}

func TestExistenceQualifiedRejectedOnSQLite(t *testing.T) {
	t.Parallel()

	d := inspectDriver(0)
	db := openInspectDriver(t, d)

	cfg := inspectBaseConfig()
	cfg.ManagedObjects = []string{"main.users"}

	_, err := prepareInspect(t, db, cfg, config.DBTypeSQLite)
	if !errors.Is(err, errs.ErrInvalidConfig) {
		t.Fatalf("Prepare() = %v, want ErrInvalidConfig", err)
	}

	if d.LastQuery() != "" {
		t.Errorf("LastQuery() = %q, want no query after rejection", d.LastQuery())
	}
}

func TestExistencePreservesQueryFailureCause(t *testing.T) {
	t.Parallel()

	permErr := errors.New("permission denied for table users")

	d := &testutil.Driver{}
	d.QueryContext = func(context.Context) (driver.Rows, error) {
		if strings.Contains(d.LastQuery(), "migrations") {
			return nil, errors.New("no such table: migrations")
		}

		return nil, permErr
	}

	db := openInspectDriver(t, d)

	_, err := prepareInspect(t, db, inspectBaseConfig(), config.DBTypeSQLite)
	if !errors.Is(err, permErr) {
		t.Fatalf("Prepare() = %v, want wrapped permission error", err)
	}

	if errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Prepare() = %v, must not classify a permission failure as ErrInvalidQuery", err)
	}

	if !strings.Contains(err.Error(), `"users"`) {
		t.Errorf("Prepare() = %v, want managed object name in the message", err)
	}
}
