package migrate

import (
	"context"
	"fmt"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

// Apply executes the plan prepared by Prepare, recording the history in the
// migrations table. It creates the history table on demand, then runs the
// steps with the transactional scope decided per dialect (E08-P01 §11):
//
//   - PostgreSQL/SQLite: one transaction per version when every file of the
//     version is transactional; a failure rolls the version back entirely and
//     leaves no record behind.
//   - MySQL, transactional file: one transaction per file; earlier files stay
//     applied.
//   - MySQL, non-transactional file: no transaction; the file is recorded
//     in_progress then applied or failed. A failure can leave this file's
//     already-committed statements in place and blocks with
//     ErrPartialMigration; nothing already committed is reverted.
//
// Apply is a no-op for a plan without steps.
func Apply(ctx context.Context, exec executor, begin Beginner, plan *Plan, cfg *config.MigrationsConfig, d dialect.Dialect) error {
	if plan == nil {
		return fmt.Errorf("%w: migration plan is nil", errs.ErrInvalidQuery)
	}

	if len(plan.Steps) == 0 {
		return nil
	}

	if exec == nil {
		return fmt.Errorf("%w: migration execution has no executor", errs.ErrInvalidQuery)
	}

	if begin == nil {
		return fmt.Errorf("%w: migration execution has no transaction starter", errs.ErrInvalidQuery)
	}

	if d == nil {
		return fmt.Errorf("%w: migration execution has no dialect", errs.ErrInvalidQuery)
	}

	if _, ok := d.(dialect.MigrationDialect); !ok {
		return fmt.Errorf("%w: dialect %s has no migration capabilities", errs.ErrInvalidQuery, d.Name())
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("krios: migrations apply: %w", err)
	}

	if err := ensureHistoryTable(ctx, exec, d); err != nil {
		return err
	}

	return applySteps(ctx, exec, begin, plan, cfg, d)
}

// ensureHistoryTable creates the history table when it is absent. It reuses
// the same existence probe as preparation: a missing table means no history.
func ensureHistoryTable(ctx context.Context, exec executor, d dialect.Dialect) error {
	_, exists, err := readHistory(ctx, exec, d)

	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	if _, err := exec.ExecContext(ctx, createHistoryDDL(d)); err != nil {
		return fmt.Errorf("krios: migrations: create history table: %w", err)
	}

	return nil
}

// applySteps runs the steps grouped by version and direction, choosing the
// transactional scope from the dialect and the per-file classification.
//
// lastApplied tracks the last version that completed entirely, never the last
// file that succeeded: a failure on the second file of a version must report
// the previous complete version, not the in-flight one (C03-P04 D19). Within a
// group the incoming complete version is passed to the partial error and the
// reached version is assigned only after the whole group succeeds.
func applySteps(ctx context.Context, exec executor, begin Beginner, plan *Plan, cfg *config.MigrationsConfig, d dialect.Dialect) error {
	md := d.(dialect.MigrationDialect)

	lastApplied := plan.CurrentVersion

	groups := groupByVersion(plan.Steps)

	for _, group := range groups {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("krios: migrations apply: %w", err)
		}

		reached := appliedAfter(group, lastApplied)

		if md.SupportsTransactionalDDL() && allTransactional(group) {
			if err := runVersionScope(ctx, begin, group, d); err != nil {
				return err
			}

			lastApplied = reached

			continue
		}

		for _, step := range group {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("krios: migrations apply: %w", err)
			}

			if step.Transactional {
				if err := runFileScope(ctx, begin, step, d); err != nil {
					return err
				}

				continue
			}

			if err := runNoTxScope(ctx, exec, step, d, cfg, lastApplied); err != nil {
				return err
			}
		}

		lastApplied = reached
	}

	return nil
}

// appliedAfter returns the version the schema reached after a successful up
// group, or after a down group the new lower version. For a down group the
// reached version is the group's version minus one; for an up group it is the
// group's highest version.
func appliedAfter(group []Step, previous int) int {
	if len(group) == 0 {
		return previous
	}

	if group[0].Direction == DirectionDown {
		return group[0].Version - 1
	}

	return group[len(group)-1].Version
}

// groupByVersion preserves the plan order and groups consecutive steps that
// share a version. Down plans run highest version first, so groups are
// naturally in execution order.
func groupByVersion(steps []Step) [][]Step {
	var groups [][]Step

	for i := 0; i < len(steps); {
		v := steps[i].Version
		j := i

		for j < len(steps) && steps[j].Version == v {
			j++
		}

		groups = append(groups, steps[i:j])
		i = j
	}

	return groups
}

// allTransactional reports whether every step of the group is transactional.
func allTransactional(group []Step) bool {
	for _, step := range group {
		if !step.Transactional {
			return false
		}
	}

	return true
}

// runVersionScope executes every file of one version inside a single
// transaction, recording each file's history in the same transaction. A
// failure rolls the whole version back, leaving no in_progress record
// (E08-P01 §11).
func runVersionScope(ctx context.Context, begin Beginner, group []Step, d dialect.Dialect) error {
	tx, err := begin.Begin(ctx)

	if err != nil {
		return fmt.Errorf("krios: migrations: begin version scope: %w", err)
	}

	for _, step := range group {
		if err := executeFile(ctx, tx, step, d); err != nil {
			return rollbackVersion(ctx, tx, step, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return commitScope(ctx, tx, fmt.Errorf("krios: migrations: commit version scope: %w", err))
	}

	return nil
}

// commitScope reports a commit failure of a migration scope. A failed commit
// does not establish the remote outcome, so the scope must not leave a
// possibly-active transaction behind: a defensive rollback is attempted and
// its outcome preserved, and the error states that the state is uncertain
// (C03-P03 D16). The commit cause is always preserved.
func commitScope(ctx context.Context, tx Tx, cause error) error {
	if rbErr := tx.Rollback(ctx); rbErr != nil {
		return fmt.Errorf("%w (state uncertain; cleanup rollback failed: %w)", cause, rbErr)
	}

	return fmt.Errorf("%w (state uncertain; transaction cleanup attempted)", cause)
}

// rollbackVersion rolls the failed version back and reports the failure. The
// rollback outcome is preserved alongside the original error.
func rollbackVersion(ctx context.Context, tx Tx, step Step, cause error) error {
	rbErr := tx.Rollback(ctx)

	if rbErr != nil {
		return fmt.Errorf("krios: migrations: version %d %s: %w (rollback failed: %w)", step.Version, step.Direction, cause, rbErr)
	}

	return fmt.Errorf("krios: migrations: version %d %s: %w", step.Version, step.Direction, cause)
}

// runFileScope executes one transactional MySQL file inside its own
// transaction, recording its history in the same transaction. Earlier files
// stay applied (E08-P01 §11).
func runFileScope(ctx context.Context, begin Beginner, step Step, d dialect.Dialect) error {
	tx, err := begin.Begin(ctx)

	if err != nil {
		return fmt.Errorf("krios: migrations: begin file scope: %w", err)
	}

	if err := executeFile(ctx, tx, step, d); err != nil {
		rbErr := tx.Rollback(ctx)

		if rbErr != nil {
			return fmt.Errorf("krios: migrations: %s: %w (rollback failed: %w)", step.File.Name(), err, rbErr)
		}

		return fmt.Errorf("krios: migrations: %s: %w", step.File.Name(), err)
	}

	if err := tx.Commit(ctx); err != nil {
		return commitScope(ctx, tx, fmt.Errorf("krios: migrations: commit %s: %w", step.File.Name(), err))
	}

	return nil
}

// runNoTxScope executes a non-transactional MySQL file without a transaction.
// The file is recorded in_progress before running and applied or failed after.
// A failure can leave this file's committed statements in place; it blocks
// with ErrPartialMigration and never reverts them (E08-P01 §11).
//
// Failure diagnosis (C03-P03 D14/D15): a body failure is reported as partial
// even when the failed-record update also fails; a markApplied failure after a
// committed body is reported as an uncertain partial state instead of a plain
// error, because the SQL already took effect without a confirmed record.
func runNoTxScope(ctx context.Context, exec executor, step Step, d dialect.Dialect, cfg *config.MigrationsConfig, lastApplied int) error {
	if err := insertInProgress(ctx, exec, d, step); err != nil {
		return fmt.Errorf("krios: migrations: record %s: %w", step.File.Name(), err)
	}

	if _, err := exec.ExecContext(ctx, step.File.Text()); err != nil {
		return partialFailure(ctx, exec, step, d, cfg, lastApplied, err)
	}

	if err := markApplied(ctx, exec, d, step); err != nil {
		return uncertainPartial(step, cfg, lastApplied, fmt.Errorf("mark %s applied: %w", step.File.Name(), err))
	}

	return nil
}

// executeFile records the step in_progress, runs the file text unchanged on
// the transaction and marks it applied. The whole sequence shares the
// transaction, so a rollback removes the history record too.
func executeFile(ctx context.Context, tx Tx, step Step, d dialect.Dialect) error {
	if err := insertInProgress(ctx, tx, d, step); err != nil {
		return fmt.Errorf("record %s: %w", step.File.Name(), err)
	}

	if _, err := tx.ExecContext(ctx, step.File.Text()); err != nil {
		return fmt.Errorf("execute %s: %w", step.File.Name(), err)
	}

	if err := markApplied(ctx, tx, d, step); err != nil {
		return fmt.Errorf("mark %s applied: %w", step.File.Name(), err)
	}

	return nil
}

// partialFailure records the step as failed and returns an ErrPartialMigration
// describing the blocked state without reverting anything already committed.
// lastApplied is the highest version applied before this step.
//
// When marking the failure also fails, the run must still block unsafe
// retries: the returned error keeps ErrPartialMigration identity and preserves
// both the original cause and the record failure (C03-P03 D14).
func partialFailure(ctx context.Context, exec executor, step Step, d dialect.Dialect, cfg *config.MigrationsConfig, lastApplied int, cause error) error {
	if err := markFailed(ctx, exec, d, step, cause); err != nil {
		recordErr := fmt.Errorf("record %s failure: %w", step.File.Name(), err)
		combined := fmt.Errorf("%w (record failure: %w)", cause, recordErr)

		return partial(step, cfg, lastApplied, StatusFailed,
			"a non-transactional file failed after committing statements and its failure record could not be updated; the state is uncertain, resolve it manually before retrying",
			combined)
	}

	return partial(step, cfg, lastApplied, StatusFailed,
		"a non-transactional file failed after committing statements; resolve the partial state before retrying",
		cause)
}

// uncertainPartial reports a committed but unrecorded state: the body ran (or
// may have run) without a confirmed history record, so a plain error would let
// the consumer retry unsafely. The returned *PartialMigrationError keeps the
// ErrPartialMigration identity and preserves the cause (C03-P03 D15).
func uncertainPartial(step Step, cfg *config.MigrationsConfig, lastApplied int, cause error) error {
	return partial(step, cfg, lastApplied, StatusInProgress,
		"a non-transactional file committed without a confirmed history record; the state is uncertain, resolve it manually before retrying",
		cause)
}

// partial builds the structured blocked state shared by the uncertain paths.
func partial(step Step, cfg *config.MigrationsConfig, lastApplied int, status, message string, cause error) error {
	target := 0

	if cfg.TargetVersion != nil {
		target = *cfg.TargetVersion
	}

	return errs.NewPartialMigrationError(
		step.Direction, target, lastApplied, step.File.Name(), status,
		message,
		cause,
	)
}
