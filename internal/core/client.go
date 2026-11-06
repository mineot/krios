package core

import (
	"context"
	"fmt"
	"math"

	"github.com/mineot/krios/internal/dialects"
)

type NewClientFunc func(Config) (*Client, error)

type Client struct {
	config Config
}

func NewClient(config Config) (*Client, error) {
	if config.Context == nil {
		return nil, makeError("Context cannot be nil")
	}

	if config.DB == nil {
		return nil, makeError("Database connection cannot be nil")
	}

	if err := checkDialect(config.Dialect); err != nil {
		return nil, err
	}

	if config.DBVersion < 1 || config.DBVersion > math.MaxInt64 {
		return nil, makeError(fmt.Sprintf("DBVersion must be between 1 and %d", math.MaxInt64))
	}

	if config.Timeout < 0 {
		return nil, makeError("Timeout cannot be negative")
	}

	if err := config.Context.Err(); err != nil {
		return nil, makeError("Context is already done")
	}

	cfg := config
	cfg.Migrations = append([]Migration(nil), config.Migrations...)
	pingCtx := cfg.Context

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(cfg.Context, cfg.Timeout)
		defer cancel()
	}

	if err := cfg.DB.PingContext(pingCtx); err != nil {
		return nil, makeError("Database ping failed")
	}

	return &Client{config: cfg}, nil

}

func checkDialect(dialect dialects.Dialect) error {
	switch dialect {
	case dialects.SQLite, dialects.Postgres, dialects.Mysql:
		return nil
	default:
		return makeError(fmt.Sprintf("Unsupported dialect %q", dialect))
	}
}

func makeError(msg string) error {
	return fmt.Errorf("%s: %s", "Krios New Client Failed", msg)
}
