package connection

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/transaction"
)

// Begin reserves the connection lifecycle before waiting for the pool/driver.
func (db *DB) Begin(ctx context.Context, opts *sql.TxOptions) (*transaction.Tx, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: context must not be nil", errs.ErrInvalidConfig)
	}

	pool, info, release, err := db.reserveTransaction()

	if err != nil {
		return nil, err
	}

	tx, err := pool.BeginTx(ctx, opts)
	
	if err != nil {
		release()
		return nil, fmt.Errorf("krios: begin transaction: %w", err)
	}

	return transaction.New(ctx, tx, info, release), nil
}
