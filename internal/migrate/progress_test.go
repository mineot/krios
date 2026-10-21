package migrate

import (
	"errors"
	"testing"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/errs"
)

// This file covers C03-P04: partial progress. A version is only complete when
// every file of the direction is applied; a partial version must block with
// ErrPartialMigration and accurate LastVersion/LastFile, both for up and down
// and with non-contiguous versions. The pure helpers are exercised directly;
// the simulated MySQL/SQLite application paths live in progress_apply_test.go.

func versionFiles(version int, up, down []string) config.MigrationVersion {
	return config.MigrationVersion{Version: version, Up: up, Down: down}
}

func rec(version int, filename, direction, status string) record {
	return record{Version: version, Filename: filename, Direction: direction, Status: status}
}

// TestEffectiveVersionRequiresAllFiles: a single applied file of a
// multi-file version must not raise the effective version.
func TestEffectiveVersionRequiresAllFiles(t *testing.T) {
	cfg := &config.MigrationsConfig{
		Versions: []config.MigrationVersion{
			versionFiles(1, []string{"m/001_a.sql", "m/001_b.sql"}, nil),
		},
	}

	// Only the first file applied: version 1 is not complete.
	partial := []record{rec(1, "m/001_a.sql", DirectionUp, StatusApplied)}

	if got := effectiveVersion(partial, cfg); got != 0 {
		t.Errorf("effectiveVersion(partial) = %d, want 0", got)
	}

	if got := lastAppliedComplete(partial, cfg); got != 0 {
		t.Errorf("lastAppliedComplete(partial) = %d, want 0", got)
	}

	// Both files applied: version 1 completes.
	complete := []record{
		rec(1, "m/001_a.sql", DirectionUp, StatusApplied),
		rec(1, "m/001_b.sql", DirectionUp, StatusApplied),
	}

	if got := effectiveVersion(complete, cfg); got != 1 {
		t.Errorf("effectiveVersion(complete) = %d, want 1", got)
	}
}

// TestEffectiveVersionNonContiguous: with versions {1,3}, a completed 3 keeps
// the effective version at 3 even though 2 is undeclared.
func TestEffectiveVersionNonContiguous(t *testing.T) {
	cfg := &config.MigrationsConfig{
		Versions: []config.MigrationVersion{
			versionFiles(1, []string{"m/001_a.sql"}, []string{"m/001_a.down.sql"}),
			versionFiles(3, []string{"m/003_a.sql"}, []string{"m/003_a.down.sql"}),
		},
	}

	records := []record{
		rec(1, "m/001_a.sql", DirectionUp, StatusApplied),
		rec(3, "m/003_a.sql", DirectionUp, StatusApplied),
	}

	if got := effectiveVersion(records, cfg); got != 3 {
		t.Errorf("effectiveVersion = %d, want 3", got)
	}

	if got := lastAppliedComplete(records, cfg); got != 3 {
		t.Errorf("lastAppliedComplete = %d, want 3", got)
	}
}

// TestDetectPartialUpReportsLastFile: a partially applied up version blocks
// with the last applied file and the last complete version.
func TestDetectPartialUpReportsLastFile(t *testing.T) {
	cfg := &config.MigrationsConfig{
		Versions: []config.MigrationVersion{
			versionFiles(1, []string{"m/001_a.sql"}, []string{"m/001_a.down.sql"}),
			versionFiles(2, []string{"m/002_a.sql", "m/002_b.sql"}, []string{"m/002_a.down.sql"}),
		},
	}

	records := []record{
		rec(1, "m/001_a.sql", DirectionUp, StatusApplied),
		rec(2, "m/002_a.sql", DirectionUp, StatusApplied),
		rec(2, "m/002_b.sql", DirectionUp, StatusFailed),
	}

	err := detectPartial(records, cfg, 2)

	if !errors.Is(err, errs.ErrPartialMigration) {
		t.Fatalf("detectPartial() = %v, want ErrPartialMigration", err)
	}

	var partial *errs.PartialMigrationError

	if !errors.As(err, &partial) {
		t.Fatalf("detectPartial() = %v, want *PartialMigrationError", err)
	}

	if partial.Direction != DirectionUp {
		t.Errorf("Direction = %q, want up", partial.Direction)
	}

	if partial.LastVersion != 1 {
		t.Errorf("LastVersion = %d, want 1", partial.LastVersion)
	}

	if partial.LastFile != "m/002_a.sql" {
		t.Errorf("LastFile = %q, want m/002_a.sql", partial.LastFile)
	}
}

// TestDetectPartialDownReportsLastFile: a partially reverted down version
// blocks with the last applied down file.
func TestDetectPartialDownReportsLastFile(t *testing.T) {
	cfg := &config.MigrationsConfig{
		Versions: []config.MigrationVersion{
			versionFiles(1, []string{"m/001_a.sql"}, []string{"m/001_a.down.sql"}),
			versionFiles(3, []string{"m/003_a.sql"}, []string{"m/003_a.down.sql", "m/003_b.down.sql"}),
		},
	}

	// Version 3 was up-applied, and its down path is partially applied.
	records := []record{
		rec(1, "m/001_a.sql", DirectionUp, StatusApplied),
		rec(3, "m/003_a.sql", DirectionUp, StatusApplied),
		rec(3, "m/003_a.down.sql", DirectionDown, StatusApplied),
	}

	err := detectPartial(records, cfg, 0)

	if !errors.Is(err, errs.ErrPartialMigration) {
		t.Fatalf("detectPartial() = %v, want ErrPartialMigration", err)
	}

	var partial *errs.PartialMigrationError

	if !errors.As(err, &partial) {
		t.Fatalf("detectPartial() = %v, want *PartialMigrationError", err)
	}

	if partial.Direction != DirectionDown {
		t.Errorf("Direction = %q, want down", partial.Direction)
	}

	if partial.LastFile != "m/003_a.down.sql" {
		t.Errorf("LastFile = %q, want m/003_a.down.sql", partial.LastFile)
	}
}

// TestDetectPartialIsIdempotentOnComplete: a fully applied multi-file version
// does not block; making progress is safe.
func TestDetectPartialIsIdempotentOnComplete(t *testing.T) {
	cfg := &config.MigrationsConfig{
		Versions: []config.MigrationVersion{
			versionFiles(1, []string{"m/001_a.sql", "m/001_b.sql"}, []string{"m/001_a.down.sql"}),
		},
	}

	records := []record{
		rec(1, "m/001_a.sql", DirectionUp, StatusApplied),
		rec(1, "m/001_b.sql", DirectionUp, StatusApplied),
	}

	if err := detectPartial(records, cfg, 1); err != nil {
		t.Fatalf("detectPartial(complete) = %v, want nil", err)
	}
}

// TestAppliedAfterDownNonContiguous: a down group reports the group's version
// minus one as the reached version.
func TestAppliedAfterDownNonContiguous(t *testing.T) {
	group := []Step{{Version: 3, Direction: DirectionDown}}

	if got := appliedAfter(group, 3); got != 2 {
		t.Errorf("appliedAfter(down 3) = %d, want 2", got)
	}

	if got := appliedAfter(group, 1); got != 2 {
		t.Errorf("appliedAfter(down 3, previous 1) = %d, want 2", got)
	}

	up := []Step{{Version: 3, Direction: DirectionUp}, {Version: 3, Direction: DirectionUp}}

	if got := appliedAfter(up, 1); got != 3 {
		t.Errorf("appliedAfter(up 3) = %d, want 3", got)
	}
}
