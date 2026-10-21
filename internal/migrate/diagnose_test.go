package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
)

// This file covers C03-P03: diagnosis and cleanup on injected failures. Every
// path must preserve the original cause and any cleanup error, must attempt to
// clean up an active transaction, and must report partial/uncertain state with
// ErrPartialMigration even when updating the history also fails. Failures are
// injected with the test fixtures; real-server evidence belongs to E10/E09.

func diagFS(t *testing.T) fs.FS {
	t.Helper()

	return fstest.MapFS{
		"m/001_users.sql": &fstest.MapFile{Data: []byte("CREATE TABLE users (id INT PRIMARY KEY);")},
	}
}

func diagStep(t *testing.T, transactional bool) Step {
	t.Helper()

	return mustScopeStep(t, diagFS(t), 1, DirectionUp, "m/001_users.sql", transactional)
}

func diagCfg(t *testing.T) *config.MigrationsConfig {
	t.Helper()

	target := 1

	return &config.MigrationsConfig{FS: diagFS(t), TargetVersion: &target}
}

// callbackExecutor is a configurable executor whose failure behaviour is
// driven by a per-query hook, so a test can fail the history record and the
// file body independently.
type callbackExecutor struct {
	execHook func(query string) error
	queries  []string
}

func (e *callbackExecutor) ExecContext(_ context.Context, query string, _ ...any) (result.Result, error) {
	e.queries = append(e.queries, query)

	if e.execHook != nil {
		return nil, e.execHook(query)
	}

	return nil, nil
}

func (*callbackExecutor) QueryContext(context.Context, string, ...any) (*scan.Rows, error) {
	return nil, errors.New("Table 'app.migrations' doesn't exist")
}

func (*callbackExecutor) QueryRowContext(context.Context, string, ...any) scan.Row {
	return scan.ErrorRow(errors.New("unexpected managed object probe"))
}

func (e *callbackExecutor) ran(sub string) bool {
	for _, q := range e.queries {
		if strings.Contains(q, sub) {
			return true
		}
	}

	return false
}

const historyInsert = "INSERT INTO"

func diagPlan(t *testing.T, transactional bool) *Plan {
	return &Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps:          []Step{diagStep(t, transactional)},
	}
}

func diagApply(t *testing.T, exec executor, begin Beginner, plan *Plan) error {
	t.Helper()

	return Apply(context.Background(), exec, begin, plan, diagCfg(t), dialect.For(config.DBTypeMySQL))
}

// TestNoTxInsertInProgressFailure: recording the initial in_progress row fails
// before the file runs. Nothing committed, so the error is a plain record
// failure with the original cause preserved.
func TestNoTxInsertInProgressFailure(t *testing.T) {
	cause := errors.New("insert failed")

	exec := &callbackExecutor{execHook: func(query string) error {
		if strings.Contains(query, historyInsert) {
			return cause
		}

		return nil
	}}

	err := diagApply(t, exec, &scopeBeginner{}, diagPlan(t, false))

	if err == nil {
		t.Fatal("Apply() = nil, want error")
	}

	if !errors.Is(err, cause) {
		t.Errorf("Apply() = %v, want cause preserved", err)
	}

	if exec.ran("CREATE TABLE users") {
		t.Errorf("file body ran after record failure: %q", exec.queries)
	}
}

// TestNoTxBodyFailureMarkFailedAlsoFails: the non-transactional body fails and
// the markFailed update also fails. The consumer must still receive
// ErrPartialMigration so the unsafe retry is blocked, with both the original
// cause and the record failure preserved.
func TestNoTxBodyFailureMarkFailedAlsoFails(t *testing.T) {
	bodyCause := errors.New("ddl failed")
	recordCause := errors.New("mark failed failed")
	inserted := false

	exec := &callbackExecutor{execHook: func(query string) error {
		switch {
		case strings.Contains(query, historyInsert):
			inserted = true
			return nil
		case strings.HasPrefix(query, "UPDATE") && strings.Contains(query, "status"):
			return recordCause
		default:
			if inserted {
				return bodyCause
			}
		}

		return nil
	}}

	err := diagApply(t, exec, &scopeBeginner{}, diagPlan(t, false))

	if !errors.Is(err, errs.ErrPartialMigration) {
		t.Fatalf("Apply() = %v, want ErrPartialMigration", err)
	}

	if !errors.Is(err, bodyCause) {
		t.Errorf("Apply() = %v, want body cause preserved", err)
	}

	if !errors.Is(err, recordCause) {
		t.Errorf("Apply() = %v, want record failure preserved", err)
	}

	var partial *errs.PartialMigrationError

	if !errors.As(err, &partial) {
		t.Fatalf("Apply() = %v, want *PartialMigrationError", err)
	}
}

// TestNoTxMarkAppliedFailure: the body committed but stamping applied failed.
// The state is uncertain (committed without an applied record); the consumer
// must receive ErrPartialMigration instead of a plain error.
func TestNoTxMarkAppliedFailure(t *testing.T) {
	recordCause := errors.New("mark applied failed")
	inserted := false

	exec := &callbackExecutor{execHook: func(query string) error {
		if strings.Contains(query, historyInsert) {
			inserted = true
			return nil
		}

		if inserted && strings.HasPrefix(query, "UPDATE") {
			return recordCause
		}

		return nil
	}}

	err := diagApply(t, exec, &scopeBeginner{}, diagPlan(t, false))

	if !errors.Is(err, errs.ErrPartialMigration) {
		t.Fatalf("Apply() = %v, want ErrPartialMigration", err)
	}

	if !errors.Is(err, recordCause) {
		t.Errorf("Apply() = %v, want record failure preserved", err)
	}
}

// TestVersionScopeRollbackOnFileFailure: a version-scope failure rolls the
// active transaction back and preserves both the cause and a rollback failure.
func TestVersionScopeRollbackOnFileFailure(t *testing.T) {
	bodyCause := errors.New("file failed")
	rollCause := errors.New("rollback failed")

	begin := &hookedBeginner{bodyCause: bodyCause, rollErr: rollCause}

	err := Apply(context.Background(), &scopeExecutor{}, begin, &Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps:          []Step{diagStep(t, true)},
	}, diagCfg(t), dialect.For(config.DBTypeSQLite))

	if err == nil {
		t.Fatal("Apply() = nil, want error")
	}

	if !errors.Is(err, bodyCause) {
		t.Errorf("Apply() = %v, want body cause preserved", err)
	}

	if !errors.Is(err, rollCause) {
		t.Errorf("Apply() = %v, want rollback failure preserved", err)
	}

	if len(begin.txs) != 1 || !begin.txs[0].rolled {
		t.Errorf("version scope did not roll back the active transaction")
	}
}

// hookedBeginner starts transactions that fail on the file body, to exercise
// the version scope's rollback path deterministically.
type hookedBeginner struct {
	bodyCause error
	rollErr   error
	txs       []*hookedTx
}

func (b *hookedBeginner) Begin(context.Context) (Tx, error) {
	tx := &hookedTx{bodyCause: b.bodyCause, rollErr: b.rollErr}
	b.txs = append(b.txs, tx)

	return tx, nil
}

type hookedTx struct {
	bodyCause error
	rollErr   error
	rolled    bool
}

func (tx *hookedTx) ExecContext(_ context.Context, query string, _ ...any) (result.Result, error) {
	if strings.Contains(query, "CREATE TABLE users") {
		return nil, tx.bodyCause
	}

	return nil, nil
}

func (*hookedTx) Commit(context.Context) error { return nil }

func (tx *hookedTx) Rollback(context.Context) error {
	tx.rolled = true

	return tx.rollErr
}

// TestFileScopeCommitFailureCleansUp: a transactional MySQL file commit fails.
// The outcome is uncertain, so Apply must attempt a defensive rollback and
// report the commit failure with context; the rollback outcome is preserved.
func TestFileScopeCommitFailureCleansUp(t *testing.T) {
	commitCause := errors.New("commit failed")
	rollCause := errors.New("rollback failed")

	begin := &scopeBeginner{commitErr: commitCause, rollErr: rollCause}

	plan := &Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps:          []Step{diagStep(t, true)},
	}

	err := Apply(context.Background(), &scopeExecutor{}, begin, plan, diagCfg(t), dialect.For(config.DBTypeMySQL))

	if err == nil {
		t.Fatal("Apply() = nil, want error")
	}

	if !errors.Is(err, commitCause) {
		t.Errorf("Apply() = %v, want commit cause preserved", err)
	}

	if !strings.Contains(err.Error(), "uncertain") && !strings.Contains(err.Error(), "incert") {
		t.Errorf("Apply() = %v, want uncertain-state context", err)
	}

	if len(begin.txs) != 1 || !begin.txs[0].rolled {
		t.Errorf("commit failure did not attempt a defensive rollback")
	}
}

// TestVersionScopeCommitFailureCleansUp mirrors the file scope for SQLite/PG.
func TestVersionScopeCommitFailureCleansUp(t *testing.T) {
	commitCause := errors.New("commit failed")

	begin := &scopeBeginner{commitErr: commitCause}

	plan := &Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps:          []Step{diagStep(t, true)},
	}

	err := Apply(context.Background(), &scopeExecutor{}, begin, plan, diagCfg(t), dialect.For(config.DBTypePostgres))

	if err == nil {
		t.Fatal("Apply() = nil, want error")
	}

	if !errors.Is(err, commitCause) {
		t.Errorf("Apply() = %v, want commit cause preserved", err)
	}

	if len(begin.txs) != 1 || !begin.txs[0].rolled {
		t.Errorf("commit failure did not attempt a defensive rollback")
	}
}

// TestBeginFailure: a Begin failure before any work returns the cause with
// scope context and never runs the file.
func TestBeginFailure(t *testing.T) {
	beginCause := fmt.Errorf("dial: %w", errors.New("connection refused"))

	begin := &scopeBeginner{beginErr: beginCause}

	plan := &Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps:          []Step{diagStep(t, true)},
	}

	err := Apply(context.Background(), &scopeExecutor{}, begin, plan, diagCfg(t), dialect.For(config.DBTypeMySQL))

	if err == nil {
		t.Fatal("Apply() = nil, want error")
	}

	if !errors.Is(err, beginCause) {
		t.Errorf("Apply() = %v, want begin cause preserved", err)
	}

	if len(begin.txs) != 0 {
		t.Errorf("Begin recorded %d tx(s) after failure", len(begin.txs))
	}
}

// TestCancelBetweenSteps: a context canceled between steps stops the run with
// a context error before the next scope starts.
func TestCancelBetweenSteps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	begin := &scopeBeginner{}

	plan := &Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps:          []Step{diagStep(t, true)},
	}

	err := Apply(ctx, &scopeExecutor{}, begin, plan, diagCfg(t), dialect.For(config.DBTypeMySQL))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Apply() = %v, want context.Canceled", err)
	}

	if len(begin.txs) != 0 {
		t.Errorf("canceled run started %d transaction(s)", len(begin.txs))
	}
}
