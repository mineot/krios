package tests

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestSQLiteEnvironment verifies the test driver and its file-backed database.
// It does not exercise Krios features, which are not implemented yet.
func TestSQLiteEnvironment(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.db")
	db := openSQLite(t, path)
	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Logf("SQLite %s", version)
	if _, err := db.Exec("CREATE TABLE environment_check (value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	want := "SQLite test environment"
	if _, err := db.Exec("INSERT INTO environment_check (value) VALUES (?)", want); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openSQLite(t, path)
	var got string
	if err := reopened.QueryRow("SELECT value FROM environment_check").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("persisted value = %q, want %q", got, want)
	}
}

// TestPrepareDevelopmentDatabase is an opt-in local setup command. Normal tests
// never open dev.db. Keeping this helper in _test.go keeps the driver test-only.
func TestPrepareDevelopmentDatabase(t *testing.T) {
	path := os.Getenv("KRIOS_DEV_DB_PATH")
	if path == "" {
		t.Skip("run make dev-db to prepare the local development database")
	}
	if os.Getenv("CI") != "" {
		t.Fatal("development database setup is local only")
	}
	db := openSQLite(t, path)
	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Logf("dev.db ready with SQLite %s; existing data preserved", version)
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close SQLite: %v", err)
		}
	})
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}
