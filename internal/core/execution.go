package core

import (
	"context"
	"database/sql"
	"fmt"
)

// executor is the Krios contract used by consumers inside the package,
// including future builders. It preserves DB/Tx guards and transaction policy.
type executor interface {
	ExecContext(context.Context, string, ...any) (Result, error)
	QueryContext(context.Context, string, ...any) (*Rows, error)
	QueryRowContext(context.Context, string, ...any) Row
}

var (
	_ executor = (*DB)(nil)
	_ executor = (*Tx)(nil)
)

// sqlExecutor is the internal execution contract satisfied by the underlying
// *sql.DB and *sql.Tx. The public methods of DB and Tx validate their own
// state (open or tx done) and then delegate through these helpers, which
// check the context, call the underlying database/sql object and apply the
// public wrappers.
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var (
	_ sqlExecutor = (*sql.DB)(nil)
	_ sqlExecutor = (*sql.Tx)(nil)
)

// execWith validates ctx, executes the query on ex and wraps the result.
// op identifies the operation in context errors ("exec" or "tx exec").
func execWith(ctx context.Context, ex sqlExecutor, op, query string, args ...any) (Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: %s: %w", op, err)
	}

	res, err := ex.ExecContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	return wrapResult(res), nil
}

// queryWith validates ctx, executes the query on ex and wraps the rows.
// op identifies the operation in context errors ("query" or "tx query").
func queryWith(ctx context.Context, ex sqlExecutor, op, query string, args ...any) (*Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: %s: %w", op, err)
	}

	r, err := ex.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	return wrapRows(r), nil
}

// queryRowWith validates ctx, executes the query on ex and wraps the row.
// Errors are deferred to Scan, including cancellation and deadline errors.
func queryRowWith(ctx context.Context, ex sqlExecutor, query string, args ...any) Row {
	if err := ctx.Err(); err != nil {
		return Row{err: err}
	}

	return wrapRow(ex.QueryRowContext(ctx, query, args...))
}
