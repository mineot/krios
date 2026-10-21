package main

import (
	"context"
	"github.com/mineot/krios"
	"path/filepath"
	"testing"
)

func TestPreexistingTableIsPreserved(t *testing.T) {
	ctx := context.Background()
	cfg := krios.Config{Type: krios.DBTypeSQLite, Driver: "sqlite", SQLite: &krios.SQLiteConfig{DBPath: filepath.Join(t.TempDir(), "existing.db")}}
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, "CREATE TABLE krios_c06_entries (marker INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "INSERT INTO krios_c06_entries VALUES (42)"); err != nil {
		t.Fatal(err)
	}
	if err = run(ctx, cfg); err == nil {
		t.Fatal("expected CREATE collision")
	}
	var marker int
	if err = db.QueryRowContext(ctx, "SELECT marker FROM krios_c06_entries").Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if marker != 42 {
		t.Fatalf("preexisting data changed: %d", marker)
	}
}
