package tests

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

func ptrInt2(i int) *int { v := i; return &v }

// TestMigrationsReproC02_R01 validates cycle 0 → 1 → 0 → 1 with reopens between steps.
func TestMigrationsReproC02_R01(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	files := migrationFiles()
	versions := migrationVersions()
	cfgBase := func(target int) *krios.MigrationsConfig {
		return &krios.MigrationsConfig{
			FS:             files,
			TargetVersion:  ptrInt2(target),
			Versions:       versions,
			ManagedObjects: []string{"users", "posts"},
		}
	}

	ctx := context.Background()

	// 0 → 1
	db, raw := openMigrations(t, path, cfgBase(1))
	plan, err := db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step1 (0->1): RunMigrations() = %v, want nil", err)
	}
	if plan == nil || plan.CurrentVersion != 0 || plan.TargetVersion != 1 || len(plan.Steps) == 0 {
		t.Fatalf("step1 plan = %#v, want 0->1 with steps", plan)
	}
	if !tableExists(t, raw, "users") {
		t.Fatal("step1: users must exist")
	}
	if tableExists(t, raw, "posts") {
		t.Fatal("step1: posts must not exist yet")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step1: %v", err)
	}

	// 1 → 0
	db, raw = openMigrations(t, path, cfgBase(0))
	plan, err = db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step2 (1->0): RunMigrations() = %v, want nil", err)
	}
	if plan == nil || plan.TargetVersion != 0 || len(plan.Steps) == 0 {
		t.Fatalf("step2 plan = %#v, want target 0 with steps", plan)
	}
	if tableExists(t, raw, "users") {
		t.Fatal("step2: users must not exist after down to 0")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step2: %v", err)
	}

	// 0 → 1 again
	db, raw = openMigrations(t, path, cfgBase(1))
	plan, err = db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step3 (0->1): RunMigrations() = %v, want nil", err)
	}
	if plan == nil || plan.TargetVersion != 1 || len(plan.Steps) == 0 {
		t.Fatalf("step3 plan = %#v, want target 1 with steps (cycle must reapply)", plan)
	}
	if !tableExists(t, raw, "users") {
		t.Fatal("step3: users must exist after reapplying 0->1")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step3: %v", err)
	}
}

// TestMigrationsReproC02_R02 validates cycle 0 → 2 → 1 → 2 with reopens.
func TestMigrationsReproC02_R02(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	files := migrationFiles()
	versions := migrationVersions()
	cfgBase := func(target int) *krios.MigrationsConfig {
		return &krios.MigrationsConfig{
			FS:             files,
			TargetVersion:  ptrInt2(target),
			Versions:       versions,
			ManagedObjects: []string{"users", "posts"},
		}
	}

	ctx := context.Background()

	// 0 → 2
	db, raw := openMigrations(t, path, cfgBase(2))
	plan, err := db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step1 (0->2): RunMigrations() = %v, want nil", err)
	}
	if plan == nil || plan.TargetVersion != 2 || len(plan.Steps) != 2 {
		t.Fatalf("step1 plan = %#v, want 0->2 with 2 steps", plan)
	}
	if !tableExists(t, raw, "users") || !tableExists(t, raw, "posts") {
		t.Fatal("step1: both tables must exist")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step1: %v", err)
	}

	// 2 → 1
	db, raw = openMigrations(t, path, cfgBase(1))
	plan, err = db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step2 (2->1): RunMigrations() = %v, want nil", err)
	}
	if plan == nil || plan.TargetVersion != 1 || len(plan.Steps) == 0 {
		t.Fatalf("step2 plan = %#v, want 2->1 with steps", plan)
	}
	if !tableExists(t, raw, "users") {
		t.Fatal("step2: users must remain")
	}
	if tableExists(t, raw, "posts") {
		t.Fatal("step2: posts must be dropped")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step2: %v", err)
	}

	// 1 → 2 again
	db, raw = openMigrations(t, path, cfgBase(2))
	plan, err = db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step3 (1->2): RunMigrations() = %v, want nil", err)
	}
	if plan == nil || plan.TargetVersion != 2 || len(plan.Steps) == 0 {
		t.Fatalf("step3 plan = %#v, want 1->2 with steps (reapply v2)", plan)
	}
	if !tableExists(t, raw, "users") || !tableExists(t, raw, "posts") {
		t.Fatal("step3: both tables must exist after reapplying v2")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step3: %v", err)
	}
}

// TestMigrationsReproC02_R03 repeats same target.
func TestMigrationsReproC02_R03(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	cfg := func(target int) *krios.MigrationsConfig {
		return &krios.MigrationsConfig{
			FS:             migrationFiles(),
			TargetVersion:  ptrInt2(target),
			Versions:       migrationVersions(),
			ManagedObjects: []string{"users", "posts"},
		}
	}

	ctx := context.Background()
	db, raw := openMigrations(t, path, cfg(1))
	if _, err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("step1: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step1: %v", err)
	}

	db, raw = openMigrations(t, path, cfg(1))
	plan, err := db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step2: %v", err)
	}
	if plan == nil || len(plan.Steps) != 0 {
		t.Fatalf("step2 plan.Steps = %d, want 0 (no-op)", len(plan.Steps))
	}
	if !tableExists(t, raw, "users") {
		t.Fatal("users must remain")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step2: %v", err)
	}
}

// TestMigrationsReproC02_R04 same as R01 but checks after reopens (API requires reopening to change target).
func TestMigrationsReproC02_R04(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "krios.db")
	files := migrationFiles()
	versions := migrationVersions()
	cfgBase := func(target int) *krios.MigrationsConfig {
		return &krios.MigrationsConfig{
			FS:             files,
			TargetVersion:  ptrInt2(target),
			Versions:       versions,
			ManagedObjects: []string{"users", "posts"},
		}
	}

	ctx := context.Background()

	// 0 → 1
	db, raw := openMigrations(t, path, cfgBase(1))
	if _, err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("step1: %v", err)
	}
	if !tableExists(t, raw, "users") {
		t.Fatal("step1: users must exist")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step1: %v", err)
	}

	// 1 → 0
	db, raw = openMigrations(t, path, cfgBase(0))
	if _, err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("step2: %v", err)
	}
	if tableExists(t, raw, "users") {
		t.Fatal("step2: users must not exist")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step2: %v", err)
	}

	// 0 → 1
	db, raw = openMigrations(t, path, cfgBase(1))
	plan, err := db.RunMigrations(ctx)
	if err != nil {
		t.Fatalf("step3: %v", err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatalf("step3 plan.Steps = %d, want > 0 (cycle must reapply)", len(plan.Steps))
	}
	if !tableExists(t, raw, "users") {
		t.Fatal("step3: users must exist")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close step3: %v", err)
	}
}
