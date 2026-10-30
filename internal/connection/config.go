// Package connection manages database pool initialization and ownership.
package connection

import (
	"context"
	"fmt"
	"time"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

// Config configures initialization of an owned database pool.
type Config struct {
	Dialect dialect.Dialect
	Driver  string
	DSN     string
	// Timeout covers ping and version detection together; zero uses ctx only.
	Timeout time.Duration
}

func (cfg Config) validate(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: context must not be nil", errs.ErrInvalidConfig)
	}

	if !dialect.Valid(cfg.Dialect) {
		return fmt.Errorf("%w: unknown dialect", errs.ErrInvalidConfig)
	}

	if cfg.Driver == "" {
		return fmt.Errorf("%w: driver must not be empty", errs.ErrInvalidConfig)
	}

	if cfg.Timeout < 0 {
		return fmt.Errorf("%w: timeout must not be negative", errs.ErrInvalidConfig)
	}

	return nil
}
