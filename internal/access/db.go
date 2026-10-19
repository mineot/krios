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
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/execute"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/transaction"
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
	sqlDB *sql.DB

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

	return &DB{sqlDB: sqlDB}, nil
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

	return transaction.New(stx), nil
}
