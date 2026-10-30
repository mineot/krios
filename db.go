package krios

import (
	"context"
	"database/sql"

	"github.com/mineot/krios/internal/connection"
)

// DB wraps a database pool. It is safe to share between goroutines.
// Obtain a usable DB through Open or Wrap.
type DB struct {
	connection *connection.DB
}

// Open creates a pool and verifies connectivity, version and capabilities.
// The caller must register the driver before calling Open.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	db, err := connection.Open(ctx, cfg)

	if err != nil {
		return nil, err
	}

	return &DB{connection: db}, nil
}

// Wrap verifies an existing pool without taking ownership of it.
// Neither initialization failure nor Close closes the supplied pool.
func Wrap(ctx context.Context, pool *sql.DB, kind Dialect) (*DB, error) {
	db, err := connection.Wrap(ctx, pool, kind)

	if err != nil {
		return nil, err
	}

	return &DB{connection: db}, nil
}

// Close invalidates this instance and closes only a pool created by Open.
// Active Krios transactions prevent closing. Repeated closing is harmless.
func (db *DB) Close() error {
	if db == nil || db.connection == nil {
		return ErrClosed
	}

	return db.connection.Close()
}
