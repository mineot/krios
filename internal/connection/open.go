package connection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Open initializes a new pool and closes it if initialization fails.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if err := cfg.validate(ctx); err != nil {
		return nil, err
	}

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: initialize database: %w", err)
	}

	pool, err := sql.Open(cfg.Driver, cfg.DSN)

	if err != nil {
		return nil, fmt.Errorf("krios: open database: %w", err)
	}

	db, err := initialize(ctx, pool, cfg.Dialect, true)

	if err != nil {
		if closeErr := pool.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("krios: close failed initialization: %w", closeErr))
		}

		return nil, err
	}

	return db, nil
}
