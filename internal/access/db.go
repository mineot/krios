// Package access opens and manages database access. It validates the
// configuration, verifies connectivity and exposes execution and transaction
// entry points.
package access

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/execute"
	"github.com/mineot/krios/internal/migrate"
	"github.com/mineot/krios/internal/query"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/sqlfile"
	"github.com/mineot/krios/internal/transaction"
	"github.com/mineot/krios/internal/write"
)

// executor is the Krios contract used by consumers inside the packages,
// including future builders. It preserves DB/Tx guards and transaction policy.
type executor interface {
	ExecContext(context.Context, string, ...any) (result.Result, error)
	QueryContext(context.Context, string, ...any) (*scan.Rows, error)
	QueryRowContext(context.Context, string, ...any) scan.Row
}

var (
	_ executor = (*DB)(nil)
	_ executor = (*transaction.Tx)(nil)
)

// DB is the database access opened by Open. It is safe for concurrent use.
type DB struct {
	sqlDB   *sql.DB
	dialect dialect.Dialect

	// migrations is the validated migration configuration retained from Open
	// so that RunMigrations needs no argument. Only this subset of the
	// configuration is kept; the connection configuration is not retained
	// (E02-P01), which is a deliberate exception scoped to migrations.
	migrations *config.MigrationsConfig

	mu     sync.Mutex
	closed bool
}

// Open validates cfg, checks that the configured driver is registered in
// database/sql, opens the access and verifies connectivity.
//
// The effective deadline is the shortest of the context deadline and
// cfg.Timeout (default 10 seconds when zero). Cancellation and deadline are
// reported as wrapped context.Canceled or context.DeadlineExceeded, without
// ErrConnectionFailed. Any resource created is released when opening fails.
// Blocking driver calls must cooperate with context cancellation; Open cannot
// interrupt non-cooperative driver preparation, connection or cleanup calls.
func Open(ctx context.Context, cfg config.Config) (*DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	d := dialect.For(cfg.Type)
	if d == nil {
		return nil, fmt.Errorf("%w: unsupported database type %q", errs.ErrInvalidConfig, cfg.Type)
	}

	if !slices.Contains(sql.Drivers(), cfg.Driver) {
		return nil, fmt.Errorf("%w: %q", errs.ErrDriverNotLoaded, cfg.Driver)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.EffectiveTimeout())
	defer cancel()

	if err := pingCtx.Err(); err != nil {
		return nil, fmt.Errorf("krios: open: %w", err)
	}

	sqlDB, err := sql.Open(cfg.Driver, config.BuildDSN(cfg))

	if err != nil {
		if ctxErr := pingCtx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("krios: open: %w", ctxErr)
		}
		return nil, fmt.Errorf("%w: %w", errs.ErrConnectionFailed, err)
	}

	if err := pingCtx.Err(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("krios: open: %w", err)
	}

	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()

		if ctxErr := pingCtx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("krios: open: %w", ctxErr)
		}

		return nil, fmt.Errorf("%w: %w", errs.ErrConnectionFailed, err)
	}

	if err := pingCtx.Err(); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf("krios: open: %w", err)
	}

	return &DB{sqlDB: sqlDB, dialect: d, migrations: cfg.Migrations}, nil
}

// Close releases the resources associated with the access. It is idempotent:
// after the first call, subsequent calls return nil.
func (db *DB) Close() error {
	db.mu.Lock()

	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	db.closed = true

	return db.sqlDB.Close()
}

func (db *DB) ensureOpen() error {
	db.mu.Lock()

	defer db.mu.Unlock()

	if db.closed {
		return errs.ErrClosed
	}

	return nil
}

// ExecContext executes a query without returning rows.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (result.Result, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	return execute.ExecWith(ctx, db.sqlDB, "exec", query, args...)
}

// QueryContext executes a query that returns rows.
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*scan.Rows, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	return execute.QueryWith(ctx, db.sqlDB, "query", query, args...)
}

// QueryRowContext executes a query that is expected to return at most one row.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) scan.Row {
	if err := db.ensureOpen(); err != nil {
		return scan.ErrorRow(err)
	}

	return execute.QueryRowWith(ctx, db.sqlDB, query, args...)
}

// Begin starts a transaction. Its context remains in effect for the lifetime
// of the transaction; cancellation causes database/sql to roll it back.
func (db *DB) Begin(ctx context.Context) (*transaction.Tx, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: begin: %w", err)
	}

	stx, err := db.sqlDB.BeginTx(ctx, nil)

	if err != nil {
		return nil, err
	}

	return transaction.New(stx, db.dialect), nil
}

// Select starts a SELECT builder for ent, rendered with the access dialect
// and executed through this access. Construction is validated on Build or on
// the first execution call, with the access guards applied unchanged.
func (db *DB) Select(ent *entity.Entity) *query.Select {
	return query.New(db, db.dialect, ent)
}

// Insert starts an INSERT builder for ent, rendered with the access dialect
// and executed through this access. Construction is validated on Build or on
// the first execution call, with the access guards applied unchanged.
func (db *DB) Insert(ent *entity.Entity) *write.Insert {
	return write.NewInsert(db, db.dialect, ent)
}

// Update starts an UPDATE builder for ent, rendered with the access dialect
// and executed through this access. At least one Where condition is required
// before execution.
func (db *DB) Update(ent *entity.Entity) *write.Update {
	return write.NewUpdate(db, db.dialect, ent)
}

// Delete starts a DELETE builder for ent, rendered with the access dialect
// and executed through this access. At least one Where condition is required
// before execution.
func (db *DB) Delete(ent *entity.Entity) *write.Delete {
	return write.NewDelete(db, db.dialect, ent)
}

// File creates a runner that executes the embedded SQL resource f through this
// access, outside any transaction. The resource text is sent unchanged with
// parameters bound separately; DB guards (ErrClosed) apply unchanged.
func (db *DB) File(f *sqlfile.SQLFile) *sqlfile.File {
	return sqlfile.NewFile(db, f)
}

// Migrations returns the migration configuration retained from Open, or nil
// when migrations were not configured. It is provided so callers can inspect
// the same configuration the access was opened with.
func (db *DB) Migrations() *config.MigrationsConfig {
	return db.migrations
}

// migrationBeginner adapts this DB to migrate.Beginner. Begin returns the
// transaction through the migrate.Tx interface, which *transaction.Tx
// satisfies structurally, so that migrate never imports the transaction
// package.
type migrationBeginner struct{ db *DB }

func (b migrationBeginner) Begin(ctx context.Context) (migrate.Tx, error) {
	tx, err := b.db.Begin(ctx)

	if err != nil {
		return nil, err
	}

	return tx, nil
}

// RunMigrations prepares the migrations configured in Open and applies the
// resulting plan, returning the plan for inspection.
//
// Preparation runs read-only inspection queries (history and managed-object
// existence) but no migration statement, and reports every configuration,
// association, history and atomicity problem before any migration statement
// runs. When a plan has
// no steps the schema is already at the target and nothing is executed. The
// plan is returned even when applying fails, so the caller can see what was
// attempted. The DB guards (ErrClosed) apply unchanged (E08-P02/P03).
func (db *DB) RunMigrations(ctx context.Context) (*migrate.Plan, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	plan, err := migrate.Prepare(ctx, db, db.migrations, db.dialect)

	if err != nil {
		return nil, err
	}

	if len(plan.Steps) == 0 {
		return plan, nil
	}

	if err := migrate.Apply(ctx, db, migrationBeginner{db: db}, plan, db.migrations, db.dialect); err != nil {
		return plan, err
	}

	return plan, nil
}
