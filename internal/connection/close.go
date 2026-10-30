package connection

import (
	"fmt"

	"github.com/mineot/krios/internal/errs"
)

// Close invalidates the handle and releases its owned pool exactly once.
func (db *DB) Close() error {
	if db == nil {
		return errs.ErrClosed
	}

	db.mu.Lock()

	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	if db.pool == nil {
		return errs.ErrClosed
	}

	if db.activeTransactions != 0 {
		return errs.ErrActiveTransactions
	}

	db.closed = true

	if db.owned {
		if err := db.pool.Close(); err != nil {
			return fmt.Errorf("krios: close database: %w", err)
		}
	}

	return nil
}
