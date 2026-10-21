package examples

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/mineot/krios"
)

// migrationsFS embeds the versioned migration files into the executable, so
// the consumer ships neither the .sql sources nor an external configuration.
// Each file holds a single statement, since Krios does not split multiple
// statements.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrationVersions declares the schema versions in execution order. The
// version number and the order come from this declaration; the numeric prefix
// of the file name is only a convention. Every version declares both
// directions, so the same configuration can advance and revert the schema.
func MigrationVersions() []krios.MigrationVersion {
	return []krios.MigrationVersion{
		{
			Version: 1,
			Up:      []string{"migrations/001_projects.up.sql"},
			Down:    []string{"migrations/001_projects.down.sql"},
		},
		{
			Version: 2,
			Up:      []string{"migrations/002_project_members.up.sql"},
			Down:    []string{"migrations/002_project_members.down.sql"},
		},
	}
}

// MigrationsConfig returns the migration configuration for the given target
// version. The target is always explicit: 2 advances the schema to the second
// version and 0 reverts every applied version.
//
// The managed objects are the tables Krios owns in this database. They let
// the library refuse to adopt a database that already has the schema without
// a migration history, instead of silently assuming it was created by Krios.
func MigrationsConfig(target int) *krios.MigrationsConfig {
	return &krios.MigrationsConfig{
		FS:             migrationsFS,
		TargetVersion:  &target,
		Versions:       MigrationVersions(),
		ManagedObjects: []string{"projects", "project_members"},
	}
}

// migrationState is the observed result of one migration phase: the plan that
// was applied, the state of the managed objects and the history rows.
type migrationState struct {
	plan    *krios.MigrationPlan
	tables  map[string]bool
	history []string
}

// Migrate runs the versioned schema through three phases against the SQLite
// file at dbPath: an advance to the latest version, a revert to version 0 and
// a reapplication of the same version. Each phase opens the access with its
// own target version, because the configuration is retained by Open and the
// target is part of it.
//
// Use a fresh dbPath between runs for a reproducible output.
func Migrate(ctx context.Context, dbPath string) error {
	advance, err := migratePhase(ctx, dbPath, 2)
	if err != nil {
		return fmt.Errorf("avanço: %w", err)
	}

	fmt.Printf("avanço: %s\n", describePlan(advance.plan))
	fmt.Printf("  objetos: %s\n", describeTables(advance.tables))
	fmt.Printf("  histórico: %d registro(s)\n", len(advance.history))

	revert, err := migratePhase(ctx, dbPath, 0)
	if err != nil {
		return fmt.Errorf("reversão: %w", err)
	}

	fmt.Printf("reversão: %s\n", describePlan(revert.plan))
	fmt.Printf("  objetos: %s\n", describeTables(revert.tables))
	fmt.Printf("  histórico: %d registro(s)\n", len(revert.history))

	reapply, err := migratePhase(ctx, dbPath, 2)
	if err != nil {
		return fmt.Errorf("reaplicação: %w", err)
	}

	fmt.Printf("reaplicação: %s\n", describePlan(reapply.plan))
	fmt.Printf("  objetos: %s\n", describeTables(reapply.tables))
	fmt.Printf("  histórico: %d registro(s)\n", len(reapply.history))

	fmt.Println("execução de migrações concluída")
	return nil
}

// migratePhase opens the database with the given target version, applies the
// migrations and observes the resulting schema and history. The access is
// closed before returning, so the next phase reopens the same file.
func migratePhase(ctx context.Context, dbPath string, target int) (migrationState, error) {
	cfg := SQLiteConfig(dbPath)
	cfg.Migrations = MigrationsConfig(target)

	db, err := krios.Open(ctx, cfg)
	if err != nil {
		return migrationState{}, err
	}

	defer db.Close()

	plan, err := db.RunMigrations(ctx)
	if err != nil {
		return migrationState{}, err
	}

	state := migrationState{plan: plan, tables: make(map[string]bool)}

	for _, object := range cfg.Migrations.ManagedObjects {
		exists, err := tableExists(ctx, db, object)
		if err != nil {
			return migrationState{}, err
		}

		state.tables[object] = exists
	}

	history, err := readMigrationHistory(ctx, db)
	if err != nil {
		return migrationState{}, err
	}

	state.history = history
	return state, nil
}

// tableExists reports whether the schema has the named table. The query reads
// SQLite's catalog, so it is specific to the dialect used by this example.
func tableExists(ctx context.Context, db *krios.DB, name string) (bool, error) {
	var count int

	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

// readMigrationHistory returns the history rows in a stable order, formatted
// as "version filename direction status".
func readMigrationHistory(ctx context.Context, db *krios.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT version, filename, direction, status FROM migrations ORDER BY version, direction, filename")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []string

	for rows.Next() {
		var (
			version   int
			filename  string
			direction string
			status    string
		)

		if err := rows.Scan(&version, &filename, &direction, &status); err != nil {
			return nil, err
		}

		out = append(out, fmt.Sprintf("%d %s %s %s", version, filename, direction, status))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// describePlan summarizes a migration plan for the narrative.
func describePlan(plan *krios.MigrationPlan) string {
	return fmt.Sprintf("%d->%d (%d passo(s))", plan.CurrentVersion, plan.TargetVersion, len(plan.Steps))
}

// describeTables summarizes the state of the managed objects.
func describeTables(tables map[string]bool) string {
	parts := make([]string, 0, len(tables))

	for _, object := range MigrationsConfig(2).ManagedObjects {
		parts = append(parts, fmt.Sprintf("%s=%t", object, tables[object]))
	}

	return strings.Join(parts, " ")
}
