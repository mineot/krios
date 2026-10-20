package migrate_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/migrate"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/sqlfile"
	"github.com/mineot/krios/internal/testutil"
)

// recordingDriver records every ExecContext query text, so tests can assert
// the history statements the execution emitted.
type recordingDriver struct {
	*testutil.Driver
	execs []string
}

func newRecordingDriver(objectCount int64, historyError bool) *recordingDriver {
	rd := &recordingDriver{Driver: &testutil.Driver{}}

	rd.Exec = func(context.Context) error {
		rd.execs = append(rd.execs, rd.LastQuery())

		return nil
	}

	rd.QueryContext = func(context.Context) (driver.Rows, error) {
		if strings.Contains(rd.LastQuery(), "migrations") {
			if historyError {
				return nil, errors.New("no such table: migrations")
			}

			return &historyRows{}, nil
		}

		return &countRows{value: objectCount}, nil
	}

	return rd
}

func (rd *recordingDriver) executed(sub string) bool {
	for _, q := range rd.execs {
		if strings.Contains(q, sub) {
			return true
		}
	}

	return false
}

// openWithMigrations opens an access.DB backed by the recording driver with
// the given migrations config.
func openWithMigrations(t *testing.T, d *testutil.Driver, cfg *config.MigrationsConfig) *access.DB {
	t.Helper()

	return openTypedWithMigrations(t, d, cfg, config.DBTypeSQLite)
}

// openTypedWithMigrations opens an access.DB over the test driver with an
// explicit database type, so dialect-specific scope paths can be exercised.
func openTypedWithMigrations(t *testing.T, d *testutil.Driver, cfg *config.MigrationsConfig, typ config.DBType) *access.DB {
	t.Helper()

	cc := testutil.Config(d)
	cc.Type = typ
	cc.SQLite = nil

	switch typ {
	case config.DBTypeMySQL:
		cc.MySQL = &config.MySQLConfig{Host: "127.0.0.1", User: "app", Password: "secret", DBName: "app"}
	case config.DBTypePostgres:
		cc.Postgres = &config.PostgresConfig{Host: "127.0.0.1", User: "app", Password: "secret", DBName: "app"}
	default:
		cc.SQLite = &config.SQLiteConfig{DBPath: "unused"}
	}

	cc.Migrations = cfg

	db, err := access.Open(context.Background(), cc)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})

	return db
}

func TestApplyCreatesHistoryTableOnDemand(t *testing.T) {
	rd := newRecordingDriver(0, true)
	db := openWithMigrations(t, rd.Driver, baseConfig())

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations() = %v", err)
	}

	if !rd.executed("CREATE TABLE IF NOT EXISTS") {
		t.Errorf("history table was not created; execs = %v", rd.execs)
	}
}

func TestApplyDoesNotRecreateHistoryTable(t *testing.T) {
	rd := newRecordingDriver(0, false)
	// History exists with nothing applied: current version 0, target 2.
	db := openWithMigrations(t, rd.Driver, baseConfig())

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations() = %v", err)
	}

	if rd.executed("CREATE TABLE IF NOT EXISTS") {
		t.Errorf("history table was recreated; execs = %v", rd.execs)
	}
}

func TestApplyRecordsInProgressAndAppliedWithChecksum(t *testing.T) {
	rd := newRecordingDriver(0, true)
	db := openWithMigrations(t, rd.Driver, baseConfig())

	if _, err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations() = %v", err)
	}

	if !rd.executed("INSERT INTO") {
		t.Errorf("in_progress insert missing; execs = %v", rd.execs)
	}

	if !rd.executed("SET status") {
		t.Errorf("applied update missing; execs = %v", rd.execs)
	}
}

func TestApplyUpPlanVersionScope(t *testing.T) {
	rd := newRecordingDriver(0, true)
	db := openWithMigrations(t, rd.Driver, baseConfig())

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("RunMigrations() = %v", err)
	}

	if len(plan.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(plan.Steps))
	}

	// PostgreSQL/SQLite: one transaction per version, both commit.
	if rd.Commits.Load() != 2 {
		t.Errorf("Commits = %d, want 2", rd.Commits.Load())
	}

	if rd.Rollbacks.Load() != 0 {
		t.Errorf("Rollbacks = %d, want 0", rd.Rollbacks.Load())
	}
}

func TestApplyVersionScopeRollsBackOnFailure(t *testing.T) {
	// The first Exec inside the version scope fails; the whole version must
	// roll back with no in_progress left behind (M05).
	rd := &recordingDriver{Driver: &testutil.Driver{}}

	rd.QueryContext = func(context.Context) (driver.Rows, error) {
		if strings.Contains(rd.LastQuery(), "migrations") {
			return nil, errors.New("no such table: migrations")
		}

		return &countRows{value: 0}, nil
	}

	var calls int

	rd.Exec = func(context.Context) error {
		// Allow the CREATE TABLE outside the transaction, then fail.
		if strings.Contains(rd.LastQuery(), "CREATE TABLE") {
			return nil
		}

		calls++
		if calls >= 1 {
			return errors.New("boom")
		}

		return nil
	}

	db := openWithMigrations(t, rd.Driver, baseConfig())

	_, err := db.RunMigrations(context.Background())
	if err == nil {
		t.Fatal("RunMigrations() = nil, want error")
	}

	if rd.Rollbacks.Load() != 1 {
		t.Errorf("Rollbacks = %d, want 1", rd.Rollbacks.Load())
	}

	if rd.Commits.Load() != 0 {
		t.Errorf("Commits = %d, want 0", rd.Commits.Load())
	}
}

func TestApplyDownPlan(t *testing.T) {
	cfg := baseConfig()
	cfg.TargetVersion = 0

	rd := &recordingDriver{Driver: &testutil.Driver{}}

	rd.QueryContext = func(context.Context) (driver.Rows, error) {
		if strings.Contains(rd.LastQuery(), "migrations") {
			return &historyRows{rows: [][]driver.Value{
				{int64(1), "m/001_users.sql", "up", migrate.StatusApplied},
				{int64(2), "m/002_posts.sql", "up", migrate.StatusApplied},
			}}, nil
		}

		return &countRows{value: 0}, nil
	}

	db := openWithMigrations(t, rd.Driver, cfg)

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("RunMigrations() = %v", err)
	}

	if len(plan.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(plan.Steps))
	}

	if plan.Steps[0].Direction != migrate.DirectionDown {
		t.Errorf("Steps[0].Direction = %q, want down", plan.Steps[0].Direction)
	}
}

func TestApplyNoStepsDoesNothing(t *testing.T) {
	rd := &recordingDriver{Driver: &testutil.Driver{}}

	rd.QueryContext = func(context.Context) (driver.Rows, error) {
		return &historyRows{rows: [][]driver.Value{
			{int64(1), "m/001_users.sql", "up", migrate.StatusApplied},
			{int64(2), "m/002_posts.sql", "up", migrate.StatusApplied},
		}}, nil
	}

	db := openWithMigrations(t, rd.Driver, baseConfig())

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("RunMigrations() = %v", err)
	}

	if len(plan.Steps) != 0 {
		t.Fatalf("len(Steps) = %d, want 0", len(plan.Steps))
	}

	if len(rd.execs) != 0 {
		t.Errorf("execs = %v, want none", rd.execs)
	}

	if rd.Commits.Load() != 0 || rd.Rollbacks.Load() != 0 {
		t.Errorf("transactions = %d/%d, want 0/0", rd.Commits.Load(), rd.Rollbacks.Load())
	}
}

func TestApplyPartialFailureBlocks(t *testing.T) {
	// MySQL: a non-transactional file (CREATE) runs without a transaction; on
	// failure it records failed and blocks with ErrPartialMigration, without
	// reverting.
	cfg := baseConfig()
	cfg.TargetVersion = 1

	rd := &recordingDriver{Driver: &testutil.Driver{}}

	rd.QueryContext = func(context.Context) (driver.Rows, error) {
		if strings.Contains(rd.LastQuery(), "migrations") {
			return nil, errors.New("no such table: migrations")
		}

		return &countRows{value: 0}, nil
	}

	rd.Exec = func(context.Context) error {
		q := rd.LastQuery()
		rd.execs = append(rd.execs, q)

		if strings.Contains(q, "CREATE TABLE") && strings.Contains(q, "migrations") {
			return nil
		}

		if strings.Contains(q, "INSERT INTO") || strings.Contains(q, "SET status") {
			return nil
		}

		// The migration file body fails.
		return errors.New("ddl failed")
	}

	db := openTypedWithMigrations(t, rd.Driver, cfg, config.DBTypeMySQL)

	_, err := db.RunMigrations(context.Background())

	if !errors.Is(err, errs.ErrPartialMigration) {
		t.Fatalf("RunMigrations() = %v, want ErrPartialMigration", err)
	}

	var partial *errs.PartialMigrationError
	if !errors.As(err, &partial) {
		t.Fatal("errors.As(PartialMigrationError) = false")
	}

	if partial.Status != migrate.StatusFailed {
		t.Errorf("Status = %q, want failed", partial.Status)
	}

	if !rd.executed("SET status") {
		t.Errorf("failed record not written; execs = %v", rd.execs)
	}

	if rd.Rollbacks.Load() != 0 {
		t.Errorf("Rollbacks = %d, want 0 (no revert of committed DDL)", rd.Rollbacks.Load())
	}
}

// --- fake Beginner/Tx to exercise scope selection and commit failures ---

type fakeTx struct {
	execErr   error
	commitErr error
	execs     []string
	committed bool
	rolled    bool
}

func (t *fakeTx) ExecContext(_ context.Context, query string, _ ...any) (result.Result, error) {
	t.execs = append(t.execs, query)

	if t.execErr != nil {
		return nil, t.execErr
	}

	return nil, nil
}

func (t *fakeTx) Commit(context.Context) error {
	if t.commitErr != nil {
		return t.commitErr
	}

	t.committed = true

	return nil
}

func (t *fakeTx) Rollback(context.Context) error {
	t.rolled = true

	return nil
}

type fakeBeginner struct {
	txs       []*fakeTx
	commitErr error
}

func (b *fakeBeginner) Begin(context.Context) (migrate.Tx, error) {
	tx := &fakeTx{commitErr: b.commitErr}
	b.txs = append(b.txs, tx)

	return tx, nil
}

// fakeExecutor satisfies the migrate executor contract for Apply. Only the
// history existence probe and the no-tx path use it.
type fakeExecutor struct {
	fail string
}

func (e *fakeExecutor) ExecContext(_ context.Context, query string, _ ...any) (result.Result, error) {
	if e.fail != "" && strings.Contains(query, e.fail) {
		return nil, errors.New("exec failed")
	}

	return nil, nil
}

func (*fakeExecutor) QueryContext(context.Context, string, ...any) (*scan.Rows, error) {
	return nil, errors.New("no such table: migrations")
}

func (*fakeExecutor) QueryRowContext(context.Context, string, ...any) scan.Row {
	return scan.ErrorRow(errors.New("no such table: migrations"))
}

func mustStep(t *testing.T, fsys fs.FS, version int, direction, name string, transactional bool) migrate.Step {
	t.Helper()

	f, err := sqlfile.NewSQLFile(fsys, name)
	if err != nil {
		t.Fatalf("NewSQLFile(%q) = %v", name, err)
	}

	return migrate.Step{Version: version, Direction: direction, File: f, Transactional: transactional}
}

func TestApplyMySQLGroupedFileScope(t *testing.T) {
	// MySQL fallback: a transactional file runs in its own file scope.
	begin := &fakeBeginner{}
	exec := &fakeExecutor{}

	plan := &migrate.Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps: []migrate.Step{
			mustStep(t, fsFixture(), 1, migrate.DirectionUp, "m/003_seed.sql", true),
		},
	}

	if err := migrate.Apply(context.Background(), exec, begin, plan, baseConfig(), dialect.For(config.DBTypeMySQL)); err != nil {
		t.Fatalf("Apply() = %v", err)
	}

	if len(begin.txs) != 1 || !begin.txs[0].committed {
		t.Fatalf("file scope = %d committed, want 1 committed", len(begin.txs))
	}
}

// TestApplyPartialStateBlockAndGuidedRecovery covers M01-M04 for a MySQL
// non-transactional file: the first run fails and records "failed" without
// reverting; the next run is blocked by ErrPartialMigration; after the record
// is resolved out of band, the run resumes to the target.
func TestApplyPartialStateBlockAndGuidedRecovery(t *testing.T) {
	cfg := baseConfig()
	cfg.TargetVersion = 1

	store := &historyStore{records: map[string]string{}}

	rd := &recordingDriver{Driver: &testutil.Driver{}}

	rd.QueryContext = func(context.Context) (driver.Rows, error) {
		if strings.Contains(rd.LastQuery(), "migrations") {
			return store.rows(), nil
		}

		return &countRows{value: 0}, nil
	}

	failNext := true

	rd.Exec = func(context.Context) error {
		q := rd.LastQuery()
		rd.execs = append(rd.execs, q)

		if strings.Contains(q, "CREATE TABLE") && strings.Contains(q, "migrations") {
			return nil
		}

		key := key("m/001_users.sql", migrate.DirectionUp)

		switch {
		case strings.Contains(q, "INSERT INTO") && strings.Contains(q, "migrations"):
			store.records[key] = migrate.StatusInProgress
		case strings.Contains(q, "SET status"):
			store.records[key] = statusArg(rd.LastArgs())
		case strings.Contains(q, "UPDATE"):
			store.records[key] = migrate.StatusApplied
		default:
			// The file body fails on the first attempt only.
			if failNext {
				failNext = false

				return errors.New("ddl failed")
			}
		}

		return nil
	}

	db := openTypedWithMigrations(t, rd.Driver, cfg, config.DBTypeMySQL)

	// First run: partial failure recorded without reverting (M01/M03).
	if _, err := db.RunMigrations(context.Background()); !errors.Is(err, errs.ErrPartialMigration) {
		t.Fatalf("first RunMigrations() = %v, want ErrPartialMigration", err)
	}

	if rd.Rollbacks.Load() != 0 {
		t.Errorf("Rollbacks = %d, want 0", rd.Rollbacks.Load())
	}

	// Second run: blocked before executing (M02).
	if _, err := db.RunMigrations(context.Background()); !errors.Is(err, errs.ErrPartialMigration) {
		t.Fatalf("blocked RunMigrations() = %v, want ErrPartialMigration", err)
	}

	// Guided recovery: resolve the failed record out of band (M04).
	store.records[key("m/001_users.sql", migrate.DirectionUp)] = migrate.StatusApplied

	plan, err := db.RunMigrations(context.Background())
	if err != nil {
		t.Fatalf("recovery RunMigrations() = %v, want nil", err)
	}

	if plan.CurrentVersion != 1 {
		t.Fatalf("plan.CurrentVersion = %d, want 1 after recovery", plan.CurrentVersion)
	}
}

// historyStore is a minimal in-memory migrations table standing in for the
// records a real server would hold. It lets a test observe a partial state and
// then resolve it out of band.
type historyStore struct {
	records map[string]string
}

func key(filename, direction string) string { return filename + "|" + direction }

// statusArg returns the status string bound as the first argument of a history
// UPDATE, matching the markApplied/markFailed statement shape.
func statusArg(args []driver.NamedValue) string {
	if len(args) == 0 {
		return ""
	}

	if s, ok := args[0].Value.(string); ok {
		return s
	}

	return ""
}

func (s *historyStore) rows() *historyRows {
	rows := &historyRows{}

	for k, status := range s.records {
		parts := strings.SplitN(k, "|", 2)
		rows.rows = append(rows.rows, []driver.Value{int64(1), parts[0], parts[1], status})
	}

	return rows
}

func TestApplyCommitFailureSurfaces(t *testing.T) {
	begin := &fakeBeginner{commitErr: errors.New("commit failed")}
	exec := &fakeExecutor{}

	plan := &migrate.Plan{
		CurrentVersion: 0,
		TargetVersion:  1,
		Steps: []migrate.Step{
			mustStep(t, fsFixture(), 1, migrate.DirectionUp, "m/001_users.sql", true),
		},
	}

	err := migrate.Apply(context.Background(), exec, begin, plan, baseConfig(), dialect.For(config.DBTypeSQLite))
	if err == nil {
		t.Fatal("Apply() = nil, want error")
	}

	if !strings.Contains(err.Error(), "commit") {
		t.Errorf("Apply() = %v, want commit context", err)
	}
}
