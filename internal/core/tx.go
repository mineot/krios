package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

// Tx represents an explicit transaction. Execution and finalization calls are
// serialized, without guaranteeing order between goroutines. This does not
// serialize consumption of returned Row/Rows: consume or close results before
// the next operation and wait for consumers before finalizing the transaction.
// Operations after finalization return ErrTxDone. The first failure observed —
// an immediate ExecContext/QueryContext error or a late read failure from
// Row.Scan or Rows iteration — is remembered and blocks Commit (see Commit).
type Tx struct {
	tx   *sql.Tx
	mu   sync.Mutex // serialize execution calls on this transaction
	done bool
	err  error // first statement or late read failure; blocks Commit (see Commit)
}

// ExecContext executes a query without returning rows on this transaction.
// The first error is remembered and blocks Commit (see Commit).
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (Result, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return nil, ErrTxDone
	}

	res, err := execWith(ctx, tx.tx, "tx exec", query, args...)
	return res, tx.recordError(err)
}

// QueryContext executes a query that returns rows on this transaction.
// The first error is remembered and blocks Commit (see Commit).
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return nil, ErrTxDone
	}

	r, err := queryWith(ctx, tx.tx, "tx query", query, args...)
	if r != nil {
		r.tx = tx
	}
	return r, tx.recordError(err)
}

// QueryRowContext executes a query that is expected to return at most one row
// on this transaction.
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return Row{err: ErrTxDone}
	}

	row := queryRowWith(ctx, tx.tx, query, args...)
	row.tx = tx
	if row.r != nil && errors.Is(row.r.Err(), sql.ErrTxDone) {
		tx.done = true
	}
	return row
}

// Commit attempts confirmation unless a failure was recorded. Recorded
// failures are the first immediate ExecContext/QueryContext error and the
// first late read failure (Row.Scan, Rows.Scan, iteration, close). In that
// case it attempts rollback even if ctx is canceled and preserves both the
// first error and any rollback error.
//
// Without a recorded failure, a canceled ctx rejects Commit before the native
// call and allows a later attempt if the transaction is still active. Context
// cannot interrupt native finalization. After a native attempt the wrapper is
// terminal, even on failure; a failure does not establish the remote outcome
// or make retrying writes safe. sql.ErrNoRows and terminal state observations
// are not recorded. The consumer should still roll back explicitly at the
// first error; blocking Commit is the safety net.
func (tx *Tx) Commit(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return ErrTxDone
	}

	if tx.err != nil {
		err := tx.err
		tx.err = nil
		tx.done = true

		if rbErr := tx.tx.Rollback(); rbErr != nil {
			return fmt.Errorf("krios: tx commit blocked by previous error: %w (rollback failed: %w)", err, normalizeTxError(rbErr))
		}

		return fmt.Errorf("krios: tx commit blocked by previous error: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("krios: tx commit: %w", err)
	}

	tx.done = true
	return normalizeTxError(tx.tx.Commit())
}

// Rollback attempts cleanup even if ctx is canceled or expired. The context
// is retained for API compatibility; it cannot interrupt native rollback.
// After the native attempt, all subsequent calls return ErrTxDone.
func (tx *Tx) Rollback(_ context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return ErrTxDone
	}

	tx.done = true
	return normalizeTxError(tx.tx.Rollback())
}

// recordError is called with tx.mu held. Native terminal errors are state
// observations, not statement failures to trigger another rollback attempt.
func (tx *Tx) recordError(err error) error {
	if errors.Is(err, sql.ErrTxDone) {
		tx.done = true
	} else if err != nil && tx.err == nil {
		tx.err = err
	}
	return normalizeTxError(err)
}

// recordLate memorizes a late read failure observed on a result produced by
// this transaction (Row.Scan, Rows.Scan, iteration or close). It is called
// with tx.mu free. nil and sql.ErrNoRows are ignored; after the transaction is
// terminal the observation has no effect. The first recorded failure blocks
// Commit.
func (tx *Tx) recordLate(err error) {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return
	}

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return
	}

	tx.recordError(err)
}
