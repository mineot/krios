package transaction

import (
	"database/sql"

	"github.com/mineot/krios/internal/dialect"
)

// CompilationInfo returns retained metadata even after transaction finalization.
// It does not authorize execution or access the driver.
func (tx *Tx) CompilationInfo() (dialect.Info, error) {
	if tx == nil {
		return dialect.Info{}, sql.ErrTxDone
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.native == nil {
		return dialect.Info{}, sql.ErrTxDone
	}
	return tx.info, nil
}
