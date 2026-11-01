package krios

import (
	"database/sql"

	"github.com/mineot/krios/internal/transaction"
)

// Tx wraps a transaction obtained through DB.Begin.
// The application must not use the same transaction concurrently.
type Tx struct {
	transaction *transaction.Tx
}

// Commit confirms the transaction. On failure it attempts rollback and returns
// the original commit error. The transaction cannot be reused afterward.
func (tx *Tx) Commit() error {
	if tx == nil || tx.transaction == nil {
		return sql.ErrTxDone
	}

	return tx.transaction.Commit()
}

// Rollback abandons the transaction. A nil, uninitialized or finished
// transaction returns sql.ErrTxDone, including after a successful Commit.
func (tx *Tx) Rollback() error {
	if tx == nil || tx.transaction == nil {
		return sql.ErrTxDone
	}

	return tx.transaction.Rollback()
}
