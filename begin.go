package krios

import (
	"context"
	"database/sql"

	"github.com/mineot/krios/internal/connection"
)

// Begin opens a transaction whose lifetime is bound to ctx.
// Nil options use database/sql defaults. Config.Timeout does not apply.
// A pending Begin prevents Close until opening fails or the transaction ends.
func (db *DB) Begin(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	var conn *connection.DB

	if db != nil {
		conn = db.connection
	}

	tx, err := conn.Begin(ctx, opts)
	
	if err != nil {
		return nil, err
	}

	return &Tx{transaction: tx}, nil
}
