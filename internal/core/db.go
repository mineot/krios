package core

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"
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
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if !slices.Contains(sql.Drivers(), cfg.Driver) {
		return nil, fmt.Errorf("%w: %q", ErrDriverNotLoaded, cfg.Driver)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()

	if err := pingCtx.Err(); err != nil {
		return nil, fmt.Errorf("krios: open: %w", err)
	}

	sqlDB, err := sql.Open(cfg.Driver, buildDSN(cfg))

	if err != nil {
		if ctxErr := pingCtx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("krios: open: %w", ctxErr)
		}
		return nil, fmt.Errorf("%w: %w", ErrConnectionFailed, err)
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

		return nil, fmt.Errorf("%w: %w", ErrConnectionFailed, err)
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
		return ErrClosed
	}

	return nil
}

// ExecContext executes a query without returning rows.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (Result, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	return execWith(ctx, db.sqlDB, "exec", query, args...)
}

// QueryContext executes a query that returns rows.
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	return queryWith(ctx, db.sqlDB, "query", query, args...)
}

// QueryRowContext executes a query that is expected to return at most one row.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	if err := db.ensureOpen(); err != nil {
		return Row{err: err}
	}

	return queryRowWith(ctx, db.sqlDB, query, args...)
}

// Begin starts a transaction. Its context remains in effect for the lifetime
// of the transaction; cancellation causes database/sql to roll it back.
func (db *DB) Begin(ctx context.Context) (*Tx, error) {
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

	return &Tx{tx: stx}, nil
}
