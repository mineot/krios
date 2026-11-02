package transaction

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mineot/krios/internal/errs"
)

// CheckCommand checks context and transaction state without retaining a lock
// or exposing the native handle. Query rechecks before execution.
func (tx *Tx) CheckCommand(ctx context.Context) error {
	_, err := tx.command(ctx)
	return err
}

// command validates inputs in contract order and snapshots the native handle.
// No Krios lock is retained during file/driver I/O or the lifetime of Rows.
func (tx *Tx) command(ctx context.Context) (*sql.Tx, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: context must not be nil", errs.ErrInvalidQuery)
	}

	if tx == nil {
		return nil, sql.ErrTxDone
	}

	tx.mu.Lock()

	defer tx.mu.Unlock()

	if tx.done || tx.native == nil {
		return nil, sql.ErrTxDone
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: execute command: %w", err)
	}

	return tx.native, nil
}

func validateSQL(sqlText string) error {
	if strings.TrimSpace(sqlText) == "" {
		return fmt.Errorf("%w: SQL must not be empty", errs.ErrInvalidQuery)
	}

	return nil
}
