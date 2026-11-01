package transaction

import (
	"database/sql"
	"fmt"
)

// Commit finalizes once and preserves the native commit error on cleanup failure.
func (tx *Tx) Commit() error {
	if tx == nil {
		return sql.ErrTxDone
	}

	tx.mu.Lock()

	defer tx.mu.Unlock()

	if tx.done || tx.native == nil {
		return sql.ErrTxDone
	}

	err := tx.native.Commit()

	if err != nil {
		// Cleanup is best effort: database/sql may already have ended the Tx.
		_ = tx.native.Rollback()
	}

	tx.finish()

	if err != nil {
		return fmt.Errorf("krios: commit transaction: %w", err)
	}

	return nil
}
