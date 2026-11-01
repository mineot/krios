package transaction

import (
	"database/sql"
	"fmt"
)

// Rollback finalizes once, also when called by the context cancellation handler.
func (tx *Tx) Rollback() error {
	if tx == nil {
		return sql.ErrTxDone
	}

	tx.mu.Lock()

	defer tx.mu.Unlock()

	if tx.done || tx.native == nil {
		return sql.ErrTxDone
	}

	err := tx.native.Rollback()
	tx.finish()

	if err != nil {
		return fmt.Errorf("krios: rollback transaction: %w", err)
	}

	return nil
}
