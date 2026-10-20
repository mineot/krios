package migrate_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/migrate"
	"github.com/mineot/krios/internal/testutil"
)

// historyRows is a multi-column driver.Rows fixture for the history SELECT.
type historyRows struct {
	rows [][]driver.Value
	next int
	err  error
}

func (r *historyRows) Columns() []string {
	return []string{"version", "filename", "direction", "status"}
}

func (r *historyRows) Close() error { return nil }

func (r *historyRows) Next(dest []driver.Value) error {
	if r.next < len(r.rows) {
		copy(dest, r.rows[r.next])
		r.next++

		return nil
	}

	if r.err != nil {
		return r.err
	}

	return io.EOF
}

func fsFixture() fstest.MapFS {
	return fstest.MapFS{
		"m/001_users.sql":      {Data: []byte("CREATE TABLE users (id INTEGER PRIMARY KEY)")},
		"m/001_users.down.sql": {Data: []byte("DROP TABLE users")},
		"m/002_posts.sql":      {Data: []byte("ALTER TABLE users ADD COLUMN name TEXT")},
		"m/002_posts.down.sql": {Data: []byte("ALTER TABLE users DROP COLUMN name")},
		"m/003_seed.sql":       {Data: []byte("INSERT INTO users (id) VALUES (1); INSERT INTO users (id) VALUES (2)")},
		"m/003_seed.down.sql":  {Data: []byte("DELETE FROM users")},
	}
}

func baseConfig() *config.MigrationsConfig {
	return &config.MigrationsConfig{
		FS:            fsFixture(),
		TargetVersion: 2,
		Versions: []config.MigrationVersion{
			{Version: 1, Up: []string{"m/001_users.sql"}, Down: []string{"m/001_users.down.sql"}},
			{Version: 2, Up: []string{"m/002_posts.sql"}, Down: []string{"m/002_posts.down.sql"}},
			{Version: 3, Up: []string{"m/003_seed.sql"}, Down: []string{"m/003_seed.down.sql"}},
		},
		ManagedObjects: []string{"users"},
	}
}

// openDriver opens an access.Driver-backed DB with the given optional query
// behaviour. history (when non-nil) serves the history SELECT; objectCount
// serves the object-existence SELECT.
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

func dialectFor(t *testing.T, typ config.DBType) dialect.Dialect {
	t.Helper()

	d := dialect.For(typ)
	if d == nil {
		t.Fatalf("dialect.For(%q) = nil", typ)
	}

	return d
}

// prepare runs migrate.Prepare directly against the DB, with the test dialect
// chosen independently from the driver fixture.
func prepare(t *testing.T, db *access.DB, cfg *config.MigrationsConfig, typ config.DBType) (*migrate.Plan, error) {
	t.Helper()

	return migrate.Prepare(context.Background(), db, cfg, dialectFor(t, typ))
}

func TestPrepareNoHistoryBuildsUpPlan(t *testing.T) {
	// History table missing, no managed objects present.
	db := openDriver(t, missingHistoryDriver(0))

	plan, err := prepare(t, db, baseConfig(), config.DBTypeSQLite)
	if err != nil {
		t.Fatalf("Prepare() = %v", err)
	}

	if plan.CurrentVersion != 0 || plan.TargetVersion != 2 {
		t.Fatalf("plan versions = (%d -> %d), want (0 -> 2)", plan.CurrentVersion, plan.TargetVersion)
	}

	if len(plan.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(plan.Steps))
	}

	for i, want := range []int{1, 2} {
		if plan.Steps[i].Version != want || plan.Steps[i].Direction != migrate.DirectionUp {
			t.Errorf("Steps[%d] = v%d/%s, want v%d/up", i, plan.Steps[i].Version, plan.Steps[i].Direction, want)
		}
	}
}

func TestPrepareNoHistoryDetectsManagedObjects(t *testing.T) {
	// History missing; the object-existence query reports the object present.
	db := openDriver(t, missingHistoryDriver(1))

	_, err := prepare(t, db, baseConfig(), config.DBTypeSQLite)

	if !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatalf("Prepare() = %v, want ErrInvalidQuery", err)
	}
}

// countRows serves a single COUNT(*) row for the object-existence SELECT. A
// value of 0 means the object is absent.
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

// missingHistoryDriver returns a driver whose first QueryContext call (the
// history SELECT) reports a missing table, and whose later calls (the
// object-existence SELECT) report the object with the given count.
func missingHistoryDriver(objectCount int64) *testutil.Driver {
	var calls int

	return &testutil.Driver{
		QueryContext: func(context.Context) (driver.Rows, error) {
			calls++

			if calls == 1 {
				return nil, errors.New("no such table: migrations")
			}

			return &countRows{value: objectCount}, nil
		},
	}
}

func TestPrepareAlreadyAtTarget(t *testing.T) {
	// history: version 2 applied; target 2.
	d := &testutil.Driver{
		QueryContext: func(context.Context) (driver.Rows, error) {
			return &historyRows{rows: [][]driver.Value{
				{int64(1), "m/001_users.sql", "up", migrate.StatusApplied},
				{int64(2), "m/002_posts.sql", "up", migrate.StatusApplied},
			}}, nil
		},
	}
	db := openDriver(t, d)

	plan, err := prepare(t, db, baseConfig(), config.DBTypeSQLite)
	if err != nil {
		t.Fatalf("Prepare() = %v", err)
	}

	if plan.CurrentVersion != 2 {
		t.Errorf("CurrentVersion = %d, want 2", plan.CurrentVersion)
	}

	if len(plan.Steps) != 0 {
		t.Fatalf("len(Steps) = %d, want 0", len(plan.Steps))
	}
}

func TestPrepareDownPlan(t *testing.T) {
	cfg := baseConfig()
	cfg.TargetVersion = 0

	d := &testutil.Driver{
		QueryContext: func(context.Context) (driver.Rows, error) {
			return &historyRows{rows: [][]driver.Value{
				{int64(1), "m/001_users.sql", "up", migrate.StatusApplied},
				{int64(2), "m/002_posts.sql", "up", migrate.StatusApplied},
			}}, nil
		},
	}
	db := openDriver(t, d)

	plan, err := prepare(t, db, cfg, config.DBTypeSQLite)
	if err != nil {
		t.Fatalf("Prepare() = %v", err)
	}

	if len(plan.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(plan.Steps))
	}

	// Highest version first.
	if plan.Steps[0].Version != 2 || plan.Steps[1].Version != 1 {
		t.Fatalf("down order = [%d, %d], want [2, 1]", plan.Steps[0].Version, plan.Steps[1].Version)
	}

	for i, s := range plan.Steps {
		if s.Direction != migrate.DirectionDown {
			t.Errorf("Steps[%d].Direction = %q, want down", i, s.Direction)
		}
	}
}

func TestPrepareUpMissingDirection(t *testing.T) {
	cfg := baseConfig()
	cfg.Versions[1].Up = nil
	cfg.TargetVersion = 2

	db := openDriver(t, missingHistoryDriver(0))

	_, err := prepare(t, db, cfg, config.DBTypeSQLite)

	if !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatalf("Prepare() = %v, want ErrInvalidQuery", err)
	}
}

func TestPrepareDownMissingDirection(t *testing.T) {
	cfg := baseConfig()
	cfg.Versions[1].Down = nil
	cfg.TargetVersion = 0

	d := &testutil.Driver{
		QueryContext: func(context.Context) (driver.Rows, error) {
			return &historyRows{rows: [][]driver.Value{
				{int64(1), "m/001_users.sql", "up", migrate.StatusApplied},
				{int64(2), "m/002_posts.sql", "up", migrate.StatusApplied},
			}}, nil
		},
	}
	db := openDriver(t, d)

	_, err := prepare(t, db, cfg, config.DBTypeSQLite)

	if !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatalf("Prepare() = %v, want ErrInvalidQuery", err)
	}
}

func TestPreparePartialMigration(t *testing.T) {
	cases := []struct {
		name   string
		status string
	}{
		{name: "in_progress", status: migrate.StatusInProgress},
		{name: "failed", status: migrate.StatusFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &testutil.Driver{
				QueryContext: func(context.Context) (driver.Rows, error) {
					return &historyRows{rows: [][]driver.Value{
						{int64(1), "m/001_users.sql", "up", migrate.StatusApplied},
						{int64(2), "m/002_posts.sql", "up", tc.status},
					}}, nil
				},
			}
			db := openDriver(t, d)

			_, err := prepare(t, db, baseConfig(), config.DBTypeSQLite)

			if !errors.Is(err, errs.ErrPartialMigration) {
				t.Fatalf("Prepare() = %v, want ErrPartialMigration", err)
			}

			var partial *errs.PartialMigrationError
			if !errors.As(err, &partial) {
				t.Fatalf("errors.As(PartialMigrationError) = false")
			}

			if partial.Status != tc.status || partial.LastFile != "m/002_posts.sql" {
				t.Errorf("partial = %+v", partial)
			}

			if partial.LastVersion != 1 {
				t.Errorf("LastVersion = %d, want 1", partial.LastVersion)
			}
		})
	}
}

func TestPrepareTransactionalScope(t *testing.T) {
	cases := []struct {
		name string
		typ  config.DBType
		file string
		want bool
	}{
		{name: "sqlite ddl", typ: config.DBTypeSQLite, file: "m/001_users.sql", want: true},
		{name: "postgres ddl", typ: config.DBTypePostgres, file: "m/001_users.sql", want: true},
		{name: "mysql ddl", typ: config.DBTypeMySQL, file: "m/001_users.sql", want: false},
		{name: "mysql dml only", typ: config.DBTypeMySQL, file: "m/003_seed.sql", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseConfig()

			switch tc.file {
			case "m/003_seed.sql":
				cfg.TargetVersion = 3
			}

			db := openDriver(t, missingHistoryDriver(0))

			plan, err := prepare(t, db, cfg, tc.typ)
			if err != nil {
				t.Fatalf("Prepare() = %v", err)
			}

			var got *migrate.Step

			for i := range plan.Steps {
				if plan.Steps[i].File.Name() == tc.file {
					got = &plan.Steps[i]
					break
				}
			}

			if got == nil {
				t.Fatalf("step for %q not found", tc.file)
			}

			if got.Transactional != tc.want {
				t.Errorf("%s Transactional = %v, want %v", tc.file, got.Transactional, tc.want)
			}
		})
	}
}

func TestPrepareMissingFile(t *testing.T) {
	cfg := baseConfig()
	cfg.TargetVersion = 1
	cfg.Versions = cfg.Versions[:1]
	cfg.Versions[0].Up = []string{"m/missing.sql"}

	db := openDriver(t, missingHistoryDriver(0))

	_, err := prepare(t, db, cfg, config.DBTypeSQLite)

	if !errors.Is(err, errs.ErrFileNotFound) {
		t.Fatalf("Prepare() = %v, want ErrFileNotFound", err)
	}
}

func TestPrepareInvalidConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.MigrationsConfig
	}{
		{name: "nil fs", cfg: func() *config.MigrationsConfig { c := baseConfig(); c.FS = nil; return c }()},
		{name: "no managed objects", cfg: func() *config.MigrationsConfig { c := baseConfig(); c.ManagedObjects = nil; return c }()},
		{name: "no versions", cfg: func() *config.MigrationsConfig { c := baseConfig(); c.Versions = nil; return c }()},
		{name: "zero version", cfg: func() *config.MigrationsConfig {
			c := baseConfig()
			c.Versions[0].Version = 0
			return c
		}()},
		{name: "duplicate version", cfg: func() *config.MigrationsConfig {
			c := baseConfig()
			c.Versions[1].Version = 1
			return c
		}()},
		{name: "unsorted versions", cfg: func() *config.MigrationsConfig {
			c := baseConfig()
			c.Versions[0], c.Versions[1] = c.Versions[1], c.Versions[0]
			return c
		}()},
		{name: "duplicate file", cfg: func() *config.MigrationsConfig {
			c := baseConfig()
			c.Versions[0].Up = []string{"m/001_users.sql", "m/001_users.sql"}
			return c
		}()},
		{name: "blank file", cfg: func() *config.MigrationsConfig {
			c := baseConfig()
			c.Versions[0].Up = []string{""}
			return c
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &testutil.Driver{}
			db := openDriver(t, d)

			_, err := prepare(t, db, tc.cfg, config.DBTypeSQLite)

			if !errors.Is(err, errs.ErrInvalidConfig) {
				t.Fatalf("Prepare() = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestPrepareNilConfig(t *testing.T) {
	d := &testutil.Driver{}
	db := openDriver(t, d)

	_, err := prepare(t, db, nil, config.DBTypeSQLite)

	if !errors.Is(err, errs.ErrInvalidConfig) {
		t.Fatalf("Prepare(nil) = %v, want ErrInvalidConfig", err)
	}
}

func TestPrepareContextCancelled(t *testing.T) {
	d := &testutil.Driver{}
	db := openDriver(t, d)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := migrate.Prepare(ctx, db, baseConfig(), dialectFor(t, config.DBTypeSQLite))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Prepare() = %v, want context.Canceled", err)
	}
}

func TestRunMigrationsNotImplemented(t *testing.T) {
	d := missingHistoryDriver(0)

	cfg := testutil.Config(d)
	cfg.Migrations = baseConfig()

	db, err := access.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()

	plan, err := db.RunMigrations(context.Background())

	if !errors.Is(err, errs.ErrMigrationsNotImplemented) {
		t.Fatalf("RunMigrations() = %v, want ErrMigrationsNotImplemented", err)
	}

	if plan == nil || len(plan.Steps) != 2 {
		t.Fatalf("RunMigrations() plan = %+v, want 2 steps", plan)
	}
}

func TestRunMigrationsNoWork(t *testing.T) {
	d := &testutil.Driver{
		QueryContext: func(context.Context) (driver.Rows, error) {
			return &historyRows{rows: [][]driver.Value{
				{int64(1), "m/001_users.sql", "up", migrate.StatusApplied},
				{int64(2), "m/002_posts.sql", "up", migrate.StatusApplied},
			}}, nil
		},
	}

	cfg := testutil.Config(d)
	cfg.Migrations = baseConfig()

	db, err := access.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("RunMigrations() = %v, want nil", err)
	}

	if len(plan.Steps) != 0 {
		t.Fatalf("len(Steps) = %d, want 0", len(plan.Steps))
	}
}
