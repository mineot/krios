// migration-cycles observes C06-P02/M08 against an isolated SQLite file.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

//go:embed sql/*.sql
var files embed.FS

func main() {
	targets := flag.String("targets", "1,0,1", "ordered target versions (0, 1, 2)")
	flag.Parse()
	if err := run(*targets); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(targets string) error {
	dir, err := os.MkdirTemp("", "krios-c06-migrations-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for _, s := range strings.Split(targets, ",") {
		target, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		if err := phase(filepath.Join(dir, "check.db"), target); err != nil {
			return err
		}
	}
	return nil
}
func phase(path string, target int) error {
	ctx := context.Background()
	cfg := krios.Config{Type: krios.DBTypeSQLite, Driver: "sqlite", SQLite: &krios.SQLiteConfig{DBPath: path}, Migrations: &krios.MigrationsConfig{
		FS: files, TargetVersion: &target, ManagedObjects: []string{"c06_parent", "c06_child"}, Versions: []krios.MigrationVersion{
			{Version: 1, Up: []string{"sql/parent.up.sql"}, Down: []string{"sql/parent.down.sql"}},
			{Version: 2, Up: []string{"sql/child.up.sql", "sql/seed.up.sql"}, Down: []string{"sql/child.down.sql"}},
		},
	}}
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	plan, applyErr := db.RunMigrations(ctx)
	fmt.Printf("target=%d plan=%+v error=%v\n", target, plan, applyErr)
	var parent, child int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='c06_parent'").Scan(&parent); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='c06_child'").Scan(&child); err != nil {
		return err
	}
	fmt.Printf("parent=%d child=%d\n", parent, child)
	rows, err := db.QueryContext(ctx, "SELECT version,filename,direction,status FROM migrations ORDER BY version,filename,direction")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var version int
		var name, direction, status string
		if err := rows.Scan(&version, &name, &direction, &status); err != nil {
			return err
		}
		fmt.Println(version, name, direction, status)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if applyErr != nil {
		return applyErr
	}
	wantParent, wantChild := 0, 0
	if target >= 1 {
		wantParent = 1
	}
	if target >= 2 {
		wantChild = 1
	}
	if parent != wantParent || child != wantChild {
		return fmt.Errorf("schema does not match target %d", target)
	}
	if target == 2 {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM c06_child WHERE id=1").Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("missing seed")
		}
	}
	return nil
}
