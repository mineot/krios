package tests

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

// This file validates the migration paths and failures of E08 against a real
// SQLite database, checking both the resulting schema and the migrations
// history table. MySQL/PostgreSQL paths are covered by simulated tests in
// internal/migrate; real-server evidence belongs to E09 (scenarios M01-M07).

// migrationFiles is the embedded-resource fixture shared by the scenarios.
func migrationFiles() fstest.MapFS {
	return fstest.MapFS{
		"m/001_users.sql":      {Data: []byte("CREATE TABLE users (id INTEGER PRIMARY KEY)")},
		"m/001_users.down.sql": {Data: []byte("DROP TABLE users")},
		"m/002_posts.sql":      {Data: []byte("CREATE TABLE posts (id INTEGER PRIMARY KEY); INSERT INTO posts (id) VALUES (1)")},
		"m/002_posts.down.sql": {Data: []byte("DROP TABLE posts")},
	}
}

// migrationVersions declares two versions with both directions.
func migrationVersions() []krios.MigrationVersion {
	return []krios.MigrationVersion{
		{Version: 1, Up: []string{"m/001_users.sql"}, Down: []string{"m/001_users.down.sql"}},
		{Version: 2, Up: []string{"m/002_posts.sql"}, Down: []string{"m/002_posts.down.sql"}},
	}
}

// openMigrations opens a SQLite DB with the given migration config and returns
// it together with a direct handle used to inspect the real schema and history.
func openMigrations(t *testing.T, path string, migrateCfg *krios.MigrationsConfig) (*krios.DB, *sql.DB) {
	t.Helper()

	cfg := sqliteConfig(path)
	cfg.Migrations = migrateCfg

	db, err := krios.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() = %v, want nil", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close DB: %v", err)
		}
	})

	return db, openSQLite(t, path)
}

// historyRow is one inspected migrations row.
type historyRow struct {
	version   int
	filename  string
	direction string
	status    string
	checksum  sql.NullString
}

// readHistoryRows reads the migrations table in a deterministic order.
func readHistoryRows(t *testing.T, db *sql.DB) []historyRow {
	t.Helper()

	rows, err := db.Query("SELECT version, filename, direction, status, checksum FROM migrations ORDER BY version, direction, filename")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}

	defer rows.Close()

	var out []historyRow

	for rows.Next() {
		var r historyRow

		if err := rows.Scan(&r.version, &r.filename, &r.direction, &r.status, &r.checksum); err != nil {
			t.Fatalf("scan migrations row: %v", err)
		}

		out = append(out, r)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migrations: %v", err)
	}

	return out
}

// tableExists reports whether a table is present in the real schema.
func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()

	var n int

	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n); err != nil {
		t.Fatalf("tableExists(%q): %v", name, err)
	}

	return n > 0
}

func TestMigrationsEmptyDatabaseAdvances(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	db, raw := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  2,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("RunMigrations() = %v, want nil", err)
	}

	if plan.CurrentVersion != 0 || plan.TargetVersion != 2 {
		t.Fatalf("plan versions = %d->%d, want 0->2", plan.CurrentVersion, plan.TargetVersion)
	}

	if len(plan.Steps) != 2 {
		t.Fatalf("plan.Steps = %d, want 2", len(plan.Steps))
	}

	if !tableExists(t, raw, "users") || !tableExists(t, raw, "posts") {
		t.Fatal("both tables must exist after advancing to version 2")
	}

	history := readHistoryRows(t, raw)
	if len(history) != 2 {
		t.Fatalf("history rows = %d, want 2", len(history))
	}

	for _, r := range history {
		if r.status != "applied" || r.direction != "up" {
			t.Fatalf("history row = %+v, want up/applied", r)
		}

		if !r.checksum.Valid || r.checksum.String == "" {
			t.Fatalf("history row = %+v, want a checksum", r)
		}
	}
}

func TestMigrationsAlreadyAtTarget(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	cfg := func() *krios.MigrationsConfig {
		return &krios.MigrationsConfig{
			FS:             migrationFiles(),
			TargetVersion:  2,
			Versions:       migrationVersions(),
			ManagedObjects: []string{"users", "posts"},
		}
	}
	db, raw := openMigrations(t, path, cfg())

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("first RunMigrations() = %v, want nil", err)
	}

	before := readHistoryRows(t, raw)

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("second RunMigrations() = %v, want nil", err)
	}

	if plan.CurrentVersion != 2 {
		t.Fatalf("plan.CurrentVersion = %d, want 2", plan.CurrentVersion)
	}

	if len(plan.Steps) != 0 {
		t.Fatalf("plan.Steps = %d, want 0 (no work)", len(plan.Steps))
	}

	if after := readHistoryRows(t, raw); len(after) != len(before) {
		t.Fatalf("history rows = %d, want unchanged %d", len(after), len(before))
	}
}

func TestMigrationsPartialRevert(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	db, raw := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  2,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("advance RunMigrations() = %v, want nil", err)
	}

	// Move the target down to version 1 and reopen with the same file store.
	raw.Close()

	db2, raw2 := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  1,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	plan, err := db2.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("down RunMigrations() = %v, want nil", err)
	}

	if plan.CurrentVersion != 2 || plan.TargetVersion != 1 || len(plan.Steps) != 1 {
		t.Fatalf("plan = %d->%d with %d steps, want 2->1 with 1", plan.CurrentVersion, plan.TargetVersion, len(plan.Steps))
	}

	if tableExists(t, raw2, "posts") {
		t.Fatal("posts must be dropped after reverting to version 1")
	}

	if !tableExists(t, raw2, "users") {
		t.Fatal("users must remain after reverting to version 1")
	}
}

func TestMigrationsRevertToZero(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	db, raw := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  2,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("advance RunMigrations() = %v, want nil", err)
	}

	raw.Close()

	db2, raw2 := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  0,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	plan, err := db2.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("revert RunMigrations() = %v, want nil", err)
	}

	if plan.TargetVersion != 0 || len(plan.Steps) != 2 {
		t.Fatalf("plan.TargetVersion = %d with %d steps, want 0 with 2", plan.TargetVersion, len(plan.Steps))
	}

	if tableExists(t, raw2, "users") || tableExists(t, raw2, "posts") {
		t.Fatal("no managed table must remain after reverting to version 0")
	}
}

func TestMigrationsFailureRollsBackSQLite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	files := migrationFiles()
	files["m/002_posts.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE posts (id INTEGER PRIMARY KEY); THIS IS NOT SQL")}

	db, raw := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             files,
		TargetVersion:  2,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	plan, err := db.RunMigrations(context.Background())
	if err == nil {
		t.Fatal("RunMigrations() = nil, want an execution error")
	}

	if plan == nil {
		t.Fatal("plan must be returned even when applying fails")
	}

	if tableExists(t, raw, "posts") {
		t.Fatal("posts must not exist after the failed version rolled back")
	}

	// Version 1 committed in its own scope, so users stays; the failed
	// version 2 must leave no history record behind.
	if !tableExists(t, raw, "users") {
		t.Fatal("users must remain: version 1 committed before the failing version")
	}

	for _, r := range readHistoryRows(t, raw) {
		if r.version == 2 {
			t.Fatalf("history row = %+v, want no version 2 record after rollback", r)
		}
	}
}

func TestMigrationsMissingDownDirection(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	db, raw := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  2,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("advance RunMigrations() = %v, want nil", err)
	}

	raw.Close()

	versions := migrationVersions()
	versions[1].Down = nil

	db2, raw2 := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  1,
		Versions:       versions,
		ManagedObjects: []string{"users", "posts"},
	})

	if _, err := db2.RunMigrations(context.Background()); err == nil {
		t.Fatal("RunMigrations() = nil, want an error for a missing down direction")
	}

	if !tableExists(t, raw2, "posts") {
		t.Fatal("posts must remain: preparation failed before executing anything")
	}
}

func TestMigrationsMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	db, raw := openMigrations(t, path, &krios.MigrationsConfig{
		FS:            migrationFiles(),
		TargetVersion: 1,
		Versions: []krios.MigrationVersion{
			{Version: 1, Up: []string{"m/does_not_exist.sql"}},
		},
		ManagedObjects: []string{"users"},
	})

	if _, err := db.RunMigrations(context.Background()); err == nil {
		t.Fatal("RunMigrations() = nil, want an error for a missing file")
	}

	if tableExists(t, raw, "migrations") {
		t.Fatal("history table must not be created when preparation fails")
	}
}

func TestMigrationsSchemaWithoutHistory(t *testing.T) {
	t.Parallel()

	t.Run("managed object present blocks", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "krios.db")
		seed := openSQLite(t, path)
		if _, err := seed.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)"); err != nil {
			t.Fatalf("seed users: %v", err)
		}
		seed.Close()

		db, raw := openMigrations(t, path, &krios.MigrationsConfig{
			FS:             migrationFiles(),
			TargetVersion:  2,
			Versions:       migrationVersions(),
			ManagedObjects: []string{"users"},
		})

		if _, err := db.RunMigrations(context.Background()); err == nil {
			t.Fatal("RunMigrations() = nil, want an error: managed object exists without history")
		}

		if tableExists(t, raw, "migrations") {
			t.Fatal("history table must not be created when the check fails")
		}
	})

	t.Run("no managed object present starts at zero", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "krios.db")
		db, raw := openMigrations(t, path, &krios.MigrationsConfig{
			FS:             migrationFiles(),
			TargetVersion:  1,
			Versions:       migrationVersions(),
			ManagedObjects: []string{"users", "posts"},
		})

		plan, err := db.RunMigrations(context.Background())
		if err != nil {
			t.Fatalf("RunMigrations() = %v, want nil", err)
		}

		if plan.CurrentVersion != 0 {
			t.Fatalf("plan.CurrentVersion = %d, want 0", plan.CurrentVersion)
		}

		if !tableExists(t, raw, "users") {
			t.Fatal("users must be created from version zero")
		}
	})
}

func TestMigrationsNotConfigured(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	db, raw := openMigrations(t, path, nil)

	if _, err := db.RunMigrations(context.Background()); err == nil {
		t.Fatal("RunMigrations() = nil, want an error when migrations are not configured")
	}

	if tableExists(t, raw, "migrations") {
		t.Fatal("history table must not be created without configuration")
	}
}

func TestMigrationsContextCancelled(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	db, raw := openMigrations(t, path, &krios.MigrationsConfig{
		FS:             migrationFiles(),
		TargetVersion:  2,
		Versions:       migrationVersions(),
		ManagedObjects: []string{"users", "posts"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := db.RunMigrations(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("RunMigrations() = %v, want context.Canceled", err)
	}

	if tableExists(t, raw, "migrations") {
		t.Fatal("history table must not be created with a cancelled context")
	}
}
