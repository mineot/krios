package examples

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

// managedObjects is the schema owned by the migration example.
var managedObjects = []string{"projects", "project_members"}

func TestMigrate(t *testing.T) {
	t.Parallel()

	if err := Migrate(context.Background(), filepath.Join(t.TempDir(), "migrate.db")); err != nil {
		t.Fatalf("Migrate() = %v", err)
	}
}

// TestMigrationsAdvanceRevertReapply walks the three phases of the migration
// example against a real SQLite file: the schema advances to the last
// version, reverts to version 0 and is reapplied. Each phase checks the
// plan, the real schema and the history table, so a phase that only reports
// success without changing anything fails the test.
func TestMigrationsAdvanceRevertReapply(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "migrations.db")

	advance, err := migratePhase(ctx, dbPath, 2)
	if err != nil {
		t.Fatalf("avanço: %v", err)
	}

	if advance.plan.CurrentVersion != 0 || advance.plan.TargetVersion != 2 || len(advance.plan.Steps) != 2 {
		t.Fatalf("plano de avanço = %s, want 0->2 com 2 passo(s)", describePlan(advance.plan))
	}

	for _, object := range managedObjects {
		if !advance.tables[object] {
			t.Errorf("tabela %q ausente após o avanço", object)
		}
	}

	if want := []string{
		"1 migrations/001_projects.up.sql up applied",
		"2 migrations/002_project_members.up.sql up applied",
	}; !slices.Equal(advance.history, want) {
		t.Errorf("histórico advance = %v, want %v", advance.history, want)
	}

	revert, err := migratePhase(ctx, dbPath, 0)
	if err != nil {
		t.Fatalf("reversão: %v", err)
	}

	if revert.plan.CurrentVersion != 2 || revert.plan.TargetVersion != 0 || len(revert.plan.Steps) != 2 {
		t.Fatalf("plano de reversão = %s, want 2->0 com 2 passo(s)", describePlan(revert.plan))
	}

	for _, object := range managedObjects {
		if revert.tables[object] {
			t.Errorf("tabela %q presente após a reversão", object)
		}
	}

	if want := []string{
		"1 migrations/001_projects.down.sql down applied",
		"1 migrations/001_projects.up.sql up applied",
		"2 migrations/002_project_members.down.sql down applied",
		"2 migrations/002_project_members.up.sql up applied",
	}; !slices.Equal(revert.history, want) {
		t.Errorf("histórico revert = %v, want %v", revert.history, want)
	}

	reapply, err := migratePhase(ctx, dbPath, 2)
	if err != nil {
		t.Fatalf("reaplicação: %v", err)
	}

	if reapply.plan.CurrentVersion != 0 || reapply.plan.TargetVersion != 2 || len(reapply.plan.Steps) != 2 {
		t.Fatalf("plano de reaplicação = %s, want 0->2 com 2 passo(s)", describePlan(reapply.plan))
	}

	for _, object := range managedObjects {
		if !reapply.tables[object] {
			t.Errorf("tabela %q ausente após a reaplicação", object)
		}
	}

	if want := []string{
		"1 migrations/001_projects.down.sql down applied",
		"1 migrations/001_projects.up.sql up applied",
		"2 migrations/002_project_members.down.sql down applied",
		"2 migrations/002_project_members.up.sql up applied",
	}; !slices.Equal(reapply.history, want) {
		t.Errorf("histórico reapply = %v, want %v", reapply.history, want)
	}
}
