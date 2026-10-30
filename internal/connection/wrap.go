package connection

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

// Wrap initializes a borrowed pool without changing its configuration or ownership.
func Wrap(ctx context.Context, pool *sql.DB, kind dialect.Dialect) (*DB, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: context must not be nil", errs.ErrInvalidConfig)
	}

	if pool == nil {
		return nil, fmt.Errorf("%w: pool must not be nil", errs.ErrInvalidConfig)
	}

	if !dialect.Valid(kind) {
		return nil, fmt.Errorf("%w: unknown dialect", errs.ErrInvalidConfig)
	}

	return initialize(ctx, pool, kind, false)
}
