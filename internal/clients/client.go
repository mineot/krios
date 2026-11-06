// Package clients validates configuration and initializes database clients.
package clients

import (
	"context"
	"fmt"
	"math"

	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

// NewCLientFunc is the signature of a client constructor.
type NewCLientFunc func(core.Config) (*Client, error)

// Client holds a validated configuration and references the caller-owned pool.
type Client struct {
	config core.Config
}

// NewClient validates config and checks connectivity before returning a client.
// It copies the migrations slice but shares the database pool and context.
func NewClient(config core.Config) (*Client, error) {
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

	// Keep later changes to the caller's migration slice out of this client.
	cfg := config
	cfg.Migrations = append([]core.Migration(nil), config.Migrations...)
	pingCtx := cfg.Context

	// A zero timeout preserves the parent context without adding a deadline.
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

// checkDialect rejects SQL dialects that the library does not support.
func checkDialect(dialect dialects.Dialect) error {
	switch dialect {
	case dialects.SQLite, dialects.Postgres, dialects.Mysql:
		return nil
	default:
		return makeError(fmt.Sprintf("Unsupported dialect %q", dialect))
	}
}

// makeError adds a consistent prefix to client initialization errors.
func makeError(msg string) error {
	return fmt.Errorf("%s: %s", "Krios New Client Failed", msg)
}
