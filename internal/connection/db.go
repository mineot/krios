package connection

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/mineot/krios/internal/dialect"
)

// DB owns synchronized lifecycle state. It must not be copied after use.
type DB struct {
	mu                 sync.Mutex
	pool               *sql.DB
	info               dialect.Info
	owned              bool
	closed             bool
	activeTransactions int // Includes pending BeginTx calls until failure or finalization.
}

func initialize(ctx context.Context, pool *sql.DB, kind dialect.Dialect, owned bool) (*DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: initialize database: %w", err)
	}

	if err := pool.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("krios: ping database: %w", err)
	}

	info, err := dialect.Detect(ctx, pool, kind)

	if err != nil {
		return nil, err
	}

	return &DB{pool: pool, info: info, owned: owned}, nil
}
