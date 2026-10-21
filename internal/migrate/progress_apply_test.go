package migrate_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/migrate"
)

// This file covers the simulated MySQL/SQLite partial-progress paths of
// C03-P04: applying the first file and failing on the next, then restarting
// and confirming the version is not treated as complete. A faithful in-memory
// history table (historyStore) reflects the library's INSERT/UPDATE/DELETE so
// the restart reads the real recorded state. Real-server evidence belongs to
// E10/E09.

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

// historyStore is an in-memory migrations table keyed by
// (version, filename, direction), mirroring the library's DELETE/INSERT/UPDATE
// sequence so a restart observes the state a real server would hold.
type historyStore struct {
	rows []histRow
}

type histRow struct {
	version   int
	filename  string
	direction string
	status    string
}

func hkey(version int, filename, direction string) string {
	return fmt.Sprintf("%d|%s|%s", version, filename, direction)
}

func (s *historyStore) upsert(version int, filename, direction, status string) {
	k := hkey(version, filename, direction)

	for i := range s.rows {
		if hkey(s.rows[i].version, s.rows[i].filename, s.rows[i].direction) == k {
			s.rows[i].status = status
			return
		}
	}

	s.rows = append(s.rows, histRow{version, filename, direction, status})
}

func (s *historyStore) deleteWhere(version int, filename, direction string) {
	kept := s.rows[:0]

	for _, r := range s.rows {
		if r.version == version && r.filename == filename && r.direction == direction {
			continue
		}

		kept = append(kept, r)
	}

	s.rows = kept
}

func (s *historyStore) snapshot() []histRow {
	out := make([]histRow, len(s.rows))
	copy(out, s.rows)

	return out
}

func (s *historyStore) statusOf(version int, filename, direction string) (string, bool) {
	k := hkey(version, filename, direction)

	for _, r := range s.rows {
		if hkey(r.version, r.filename, r.direction) == k {
			return r.status, true
		}
	}

	return "", false
}

func (s *historyStore) rowsDriver() *historyRows {
	rows := &historyRows{}

	for _, r := range s.rows {
		rows.rows = append(rows.rows, []driver.Value{
			int64(r.version), r.filename, r.direction, r.status,
		})
	}

	return rows
}

// progressDriver is a buffering database/sql driver that serves a live
// historyStore for the history SELECT and mutates it on the library's history
// writes. Unlike testutil.Driver its transactions are faithful: Begin snapshots
// the store, Rollback restores it and Commit keeps the mutations, so an
// in_progress record written inside a rolled-back transaction disappears
// exactly as on a real server. bodyErr, when non-nil, is returned for the body
// of the named file (matched by its exact text), so a test can fail a specific
// file deterministically.
type progressDriver struct {
	store       *historyStore
	bodyText    string
	bodyErr     error
	historyFail func(query string) error

	mu       sync.Mutex
	execs    []string
	snapshot []histRow
}

func newProgressDriver(store *historyStore) *progressDriver {
	return &progressDriver{store: store}
}

var progressDriverSeq atomic.Int64

// openProgressDriver registers pd as a unique SQLite driver and returns a
// config that uses it with the requested DBType.
func openProgressDriver(t *testing.T, pd *progressDriver, cfg *config.MigrationsConfig, typ config.DBType) *access.DB {
	t.Helper()

	name := fmt.Sprintf("krios_progress_%d", progressDriverSeq.Add(1))
	sql.Register(name, &progressRegistered{pd})

	cc := config.Config{Type: typ, Driver: name}

	switch typ {
	case config.DBTypeMySQL:
		cc.MySQL = &config.MySQLConfig{Host: "127.0.0.1", User: "app", Password: "secret", DBName: "app"}
	case config.DBTypePostgres:
		cc.Postgres = &config.PostgresConfig{Host: "127.0.0.1", User: "app", Password: "secret", DBName: "app"}
	default:
		cc.SQLite = &config.SQLiteConfig{DBPath: "unused"}
	}

	cc.Migrations = cfg

	db, err := access.Open(context.Background(), cc)
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

// progressRegistered adapts progressDriver to driver.Driver.
type progressRegistered struct{ pd *progressDriver }

func (r *progressRegistered) Open(string) (driver.Conn, error) { return &progressConn{pd: r.pd}, nil }

type progressConn struct{ pd *progressDriver }

func (c *progressConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}

func (c *progressConn) Close() error { return nil }

func (c *progressConn) Begin() (driver.Tx, error) {
	c.pd.mu.Lock()
	c.pd.snapshot = c.pd.store.snapshot()
	c.pd.mu.Unlock()

	return progressTx{pd: c.pd}, nil
}

func (c *progressConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.pd.exec(query, args)
}

func (c *progressConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "migrations") {
		c.pd.mu.Lock()
		defer c.pd.mu.Unlock()

		return c.pd.store.rowsDriver(), nil
	}

	return &countRows{value: 0}, nil
}

type progressTx struct{ pd *progressDriver }

func (tx progressTx) Commit() error {
	tx.pd.mu.Lock()
	defer tx.pd.mu.Unlock()

	tx.pd.snapshot = nil

	return nil
}

func (tx progressTx) Rollback() error {
	pd := tx.pd

	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.store.rows = pd.snapshot
	pd.snapshot = nil

	return nil
}

// exec applies the driver hooks and records the statement. History writes
// mutate the store directly; a surrounding transaction undoes them on
// Rollback via the Begin snapshot.
func (pd *progressDriver) exec(query string, args []driver.NamedValue) (driver.Result, error) {
	pd.mu.Lock()
	pd.execs = append(pd.execs, query)
	pd.mu.Unlock()

	if pd.historyFail != nil {
		if err := pd.historyFail(query); err != nil {
			return nil, err
		}
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()

	switch {
	case strings.Contains(query, "CREATE TABLE") && strings.Contains(query, "migrations"):
		// no-op
	case strings.HasPrefix(query, "DELETE FROM") && strings.Contains(query, "migrations"):
		v, name, dir := deleteArgs(args)
		pd.store.deleteWhere(v, name, dir)
	case strings.Contains(query, "INSERT INTO") && strings.Contains(query, "migrations"):
		v, name, dir := insertArgs(args)
		pd.store.upsert(v, name, dir, migrate.StatusInProgress)
	case strings.HasPrefix(query, "UPDATE") && strings.Contains(query, "status"):
		v, name, dir, status := updateArgs(args)
		pd.store.upsert(v, name, dir, status)
	}

	if pd.bodyErr != nil && pd.bodyText != "" && strings.Contains(query, pd.bodyText) {
		return nil, pd.bodyErr
	}

	return driver.RowsAffected(1), nil
}

func deleteArgs(args []driver.NamedValue) (int, string, string) {
	version := 0

	if len(args) > 0 {
		if v, ok := args[0].Value.(int64); ok {
			version = int(v)
		}
	}

	name := ""

	if len(args) > 1 {
		if s, ok := args[1].Value.(string); ok {
			name = s
		}
	}

	dir := ""

	if len(args) > 2 {
		if s, ok := args[2].Value.(string); ok {
			dir = s
		}
	}

	return version, name, dir
}

// insertArgs reads version, filename and direction from the in_progress INSERT
// parameters (version, filename, direction, status, checksum).
func insertArgs(args []driver.NamedValue) (int, string, string) {
	version, name, dir := 0, "", ""

	if len(args) > 0 {
		if v, ok := args[0].Value.(int64); ok {
			version = int(v)
		}
	}

	if len(args) > 1 {
		if s, ok := args[1].Value.(string); ok {
			name = s
		}
	}

	if len(args) > 2 {
		if s, ok := args[2].Value.(string); ok {
			dir = s
		}
	}

	return version, name, dir
}

// updateArgs reads status and the identity columns from a history UPDATE
// (status, applied_at/error, version, filename, direction).
func updateArgs(args []driver.NamedValue) (int, string, string, string) {
	status := ""

	if len(args) > 0 {
		if s, ok := args[0].Value.(string); ok {
			status = s
		}
	}

	version, name, dir := 0, "", ""

	if len(args) > 2 {
		if v, ok := args[2].Value.(int64); ok {
			version = int(v)
		}
	}

	if len(args) > 3 {
		if s, ok := args[3].Value.(string); ok {
			name = s
		}
	}

	if len(args) > 4 {
		if s, ok := args[4].Value.(string); ok {
			dir = s
		}
	}

	return version, name, dir, status
}

// progressFS declares two versions with two up files each, so a partial
// version is observable. Version 3 uses transactional DML files, to exercise
// the MySQL file-scope (transactional) path alongside the non-transactional
// DDL path of versions 1 and 2.
func progressFS() fstest.MapFS {
	return fstest.MapFS{
		"m/001_a.sql":      {Data: []byte("CREATE TABLE users (id INTEGER PRIMARY KEY)")},
		"m/001_b.sql":      {Data: []byte("ALTER TABLE users ADD COLUMN name TEXT")},
		"m/001_a.down.sql": {Data: []byte("DROP TABLE users")},
		"m/001_b.down.sql": {Data: []byte("ALTER TABLE users DROP COLUMN name")},
		"m/002_a.sql":      {Data: []byte("CREATE TABLE posts (id INTEGER PRIMARY KEY)")},
		"m/002_b.sql":      {Data: []byte("CREATE TABLE comments (id INTEGER PRIMARY KEY)")},
		"m/002_a.down.sql": {Data: []byte("DROP TABLE posts")},
		"m/002_b.down.sql": {Data: []byte("DROP TABLE comments")},
		"m/003_a.sql":      {Data: []byte("INSERT INTO users (id) VALUES (1)")},
		"m/003_b.sql":      {Data: []byte("UPDATE users SET name = 'x'")},
		"m/003_a.down.sql": {Data: []byte("DELETE FROM users WHERE id = 1")},
		"m/003_b.down.sql": {Data: []byte("UPDATE users SET name = NULL")},
	}
}

func progressConfig(target int) *config.MigrationsConfig {
	return &config.MigrationsConfig{
		FS:            progressFS(),
		TargetVersion: &target,
		Versions: []config.MigrationVersion{
			{Version: 1, Up: []string{"m/001_a.sql", "m/001_b.sql"}, Down: []string{"m/001_a.down.sql", "m/001_b.down.sql"}},
			{Version: 2, Up: []string{"m/002_a.sql", "m/002_b.sql"}, Down: []string{"m/002_a.down.sql", "m/002_b.down.sql"}},
			{Version: 3, Up: []string{"m/003_a.sql", "m/003_b.sql"}, Down: []string{"m/003_a.down.sql", "m/003_b.down.sql"}},
		},
		ManagedObjects: []string{"users"},
	}
}

func openProgress(t *testing.T, pd *progressDriver, cfg *config.MigrationsConfig, typ config.DBType) *access.DB {
	t.Helper()

	return openProgressDriver(t, pd, cfg, typ)
}

func errorsIs(err, target error) bool { return errors.Is(err, target) }

func asPartial(err error, target **errs.PartialMigrationError) bool {
	return errors.As(err, target)
}

func sameSnapshot(a, b []histRow) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// TestMySQLPartialNoTxRestartBlocks: MySQL non-transactional, version 2 with
// two DDL files. The first file commits, the second fails. On restart the
// version is not treated as complete: ErrPartialMigration with LastFile at the
// failed file and LastVersion at the last complete version.
func TestMySQLPartialNoTxRestartBlocks(t *testing.T) {
	store := &historyStore{}

	pd := newProgressDriver(store)
	pd.bodyText = "CREATE TABLE comments"
	pd.bodyErr = fmt.Errorf("second file failed")

	cfg := progressConfig(2)
	db := openProgress(t, pd, cfg, config.DBTypeMySQL)

	_, err := db.RunMigrations(context.Background())

	if !errorsIs(err, errs.ErrPartialMigration) {
		t.Fatalf("RunMigrations() = %v, want ErrPartialMigration", err)
	}

	var partial *errs.PartialMigrationError

	if !asPartial(err, &partial) {
		t.Fatalf("RunMigrations() = %v, want *PartialMigrationError", err)
	}

	if partial.LastFile != "m/002_b.sql" {
		t.Errorf("LastFile = %q, want m/002_b.sql", partial.LastFile)
	}

	if partial.LastVersion != 1 {
		t.Errorf("LastVersion = %d, want 1", partial.LastVersion)
	}

	// Restart: the same store still holds the in_progress/failed record, so
	// the run is blocked before executing anything.
	before := len(pd.execs)
	beforeStore := store.snapshot()

	plan, err := db.RunMigrations(context.Background())

	if !errorsIs(err, errs.ErrPartialMigration) {
		t.Fatalf("restart RunMigrations() = %v, want ErrPartialMigration", err)
	}

	if plan != nil {
		t.Errorf("restart plan = %+v, want nil on blocked preparation", plan)
	}

	if len(pd.execs) != before {
		t.Errorf("restart executed %d new statements, want 0", len(pd.execs)-before)
	}

	if !sameSnapshot(beforeStore, store.snapshot()) {
		t.Errorf("restart mutated the history; before=%v after=%v", beforeStore, store.snapshot())
	}
}

// TestMySQLPartialFileScopeRestartBlocks: MySQL transactional file scope,
// version 3 with two DML files. The first file commits its own record; the
// second fails and its transaction rolls back so no record survives for it. A
// restart detects the partial version and blocks with ErrPartialMigration.
func TestMySQLPartialFileScopeRestartBlocks(t *testing.T) {
	store := &historyStore{}

	pd := newProgressDriver(store)
	pd.bodyText = "UPDATE users SET name = 'x'"
	pd.bodyErr = fmt.Errorf("second file failed")

	cfg := progressConfig(3)
	db := openProgress(t, pd, cfg, config.DBTypeMySQL)

	_, err := db.RunMigrations(context.Background())

	if errorsIs(err, errs.ErrPartialMigration) {
		t.Fatalf("RunMigrations() = %v, want a plain error for the transactional file failure", err)
	}

	if !strings.Contains(err.Error(), "second file failed") {
		t.Fatalf("RunMigrations() = %v, want cause preserved", err)
	}

	if status, ok := store.statusOf(3, "m/003_a.sql", "up"); !ok || status != migrate.StatusApplied {
		t.Errorf("version 3 file a status = %q (present=%v), want %q", status, ok, migrate.StatusApplied)
	}

	if _, ok := store.statusOf(3, "m/003_b.sql", "up"); ok {
		t.Errorf("version 3 file b left a record, want the failed transaction rolled back")
	}

	// Restart: the partial version 3 blocks, and the cause is reported.
	plan, err := db.RunMigrations(context.Background())

	if !errorsIs(err, errs.ErrPartialMigration) {
		t.Fatalf("restart RunMigrations() = %v, want ErrPartialMigration", err)
	}

	if plan != nil {
		t.Errorf("restart plan = %+v, want nil", plan)
	}

	var partial *errs.PartialMigrationError

	if !asPartial(err, &partial) {
		t.Fatalf("restart RunMigrations() = %v, want *PartialMigrationError", err)
	}

	if partial.LastFile != "m/003_a.sql" {
		t.Errorf("restart LastFile = %q, want m/003_a.sql (last applied of the partial prefix)", partial.LastFile)
	}

	if partial.LastVersion != 2 {
		t.Errorf("restart LastVersion = %d, want 2", partial.LastVersion)
	}
}

// TestSQLiteVersionScopeRollsBackWholeVersion: on SQLite PostgreSQL-style
// transactional DDL makes an all-transactional version run in a single
// transaction, so a mid-version failure is atomic: the whole version rolls
// back, no partial state exists, version 1 stays applied, and a restart resumes
// cleanly instead of blocking.
func TestSQLiteVersionScopeRollsBackWholeVersion(t *testing.T) {
	store := &historyStore{}

	pd := newProgressDriver(store)
	pd.bodyText = "CREATE TABLE comments"
	pd.bodyErr = fmt.Errorf("second file failed")

	cfg := progressConfig(2)
	db := openProgress(t, pd, cfg, config.DBTypeSQLite)

	_, err := db.RunMigrations(context.Background())

	if errorsIs(err, errs.ErrPartialMigration) {
		t.Fatalf("RunMigrations() = %v, want a plain error (whole version rolled back)", err)
	}

	if !strings.Contains(err.Error(), "second file failed") {
		t.Fatalf("RunMigrations() = %v, want cause preserved", err)
	}

	if status, ok := store.statusOf(1, "m/001_a.sql", "up"); !ok || status != migrate.StatusApplied {
		t.Errorf("version 1 file a status = %q (present=%v), want %q", status, ok, migrate.StatusApplied)
	}

	if status, ok := store.statusOf(1, "m/001_b.sql", "up"); !ok || status != migrate.StatusApplied {
		t.Errorf("version 1 file b status = %q (present=%v), want %q", status, ok, migrate.StatusApplied)
	}

	for _, name := range []string{"m/002_a.sql", "m/002_b.sql"} {
		if status, ok := store.statusOf(2, name, "up"); ok {
			t.Errorf("version 2 %s left a record (%q), want the whole version rolled back", name, status)
		}
	}

	// Restart: no partial state, so the run resumes without blocking.
	pd.bodyErr = nil

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("restart RunMigrations() = %v, want nil", err)
	}

	if status, ok := store.statusOf(2, "m/002_a.sql", "up"); !ok || status != migrate.StatusApplied {
		t.Errorf("restart version 2 file a status = %q (present=%v), want %q", status, ok, migrate.StatusApplied)
	}

	if status, ok := store.statusOf(2, "m/002_b.sql", "up"); !ok || status != migrate.StatusApplied {
		t.Errorf("restart version 2 file b status = %q (present=%v), want %q", status, ok, migrate.StatusApplied)
	}
}

// TestGuidedRecoveryAfterPartial: reproduces the M01-M04 guided-recovery flow.
// A run fails mid-version (M01), a second run is blocked (M02), an operator
// resolves the state out-of-band by marking the failed file applied (M03), and
// the next run completes and is idempotent (M04).
func TestGuidedRecoveryAfterPartial(t *testing.T) {
	store := &historyStore{}

	pd := newProgressDriver(store)
	pd.bodyText = "CREATE TABLE comments"
	pd.bodyErr = fmt.Errorf("second file failed")

	cfg := progressConfig(2)
	db := openProgress(t, pd, cfg, config.DBTypeMySQL)

	// M01: the first run fails mid-version.
	if _, err := db.RunMigrations(context.Background()); !errorsIs(err, errs.ErrPartialMigration) {
		t.Fatalf("M01 RunMigrations() = %v, want ErrPartialMigration", err)
	}

	// M02: a retry is blocked.
	if _, err := db.RunMigrations(context.Background()); !errorsIs(err, errs.ErrPartialMigration) {
		t.Fatalf("M02 RunMigrations() = %v, want ErrPartialMigration", err)
	}

	// M03: out-of-band resolution marks the failed file applied.
	store.upsert(2, "m/002_b.sql", "up", migrate.StatusApplied)

	// M04: the next run observes version 2 complete and is idempotent.
	pd.bodyErr = nil

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("M04 RunMigrations() = %v, want nil", err)
	}

	if plan == nil || len(plan.Steps) != 0 {
		t.Errorf("M04 plan = %+v, want an empty up plan", plan)
	}

	if status, ok := store.statusOf(2, "m/002_b.sql", "up"); !ok || status != migrate.StatusApplied {
		t.Errorf("M04 version 2 file b status = %q (present=%v), want %q", status, ok, migrate.StatusApplied)
	}
}
