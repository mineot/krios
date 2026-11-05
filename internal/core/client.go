package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
)

type Dialect string

const (
	DialectSQLite3    Dialect = "sqlite3"
	DialectPostgreSQL Dialect = "postgres"
	DialectMySQL      Dialect = "mysql"
)

var (
	ErrInvalidConfig    = errors.New("krios: invalid configuration")
	ErrInvalidQuery     = errors.New("krios: invalid query")
	ErrInvalidMigration = errors.New("krios: invalid migration")
)

type Config struct {
	Db         *sql.DB
	DbVersion  uint64
	Dialect    Dialect
	Timeout    time.Duration
	Context    context.Context
	Migrations []Migration
}

type Migration struct {
	Version          uint64
	Up               string
	Down             string
	NonTransactional bool
}

type Client struct {
	config Config
}

func New(config Config) (*Client, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	config.Migrations = slices.Clone(config.Migrations)
	client := &Client{config: config}
	ctx, cancel := client.operationContext()

	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: ping: %w", err)
	}

	if err := config.Db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("krios: ping: %w", err)
	}

	return client, nil
}

func validateConfig(config Config) error {
	switch {
	case config.Db == nil:
		return fmt.Errorf("%w: Db must not be nil", ErrInvalidConfig)
	case config.Context == nil:
		return fmt.Errorf("%w: Context must not be nil", ErrInvalidConfig)
	case config.Dialect != DialectSQLite3 && config.Dialect != DialectPostgreSQL && config.Dialect != DialectMySQL:
		return fmt.Errorf("%w: unsupported Dialect", ErrInvalidConfig)
	case config.Timeout < 0:
		return fmt.Errorf("%w: Timeout must not be negative", ErrInvalidConfig)
	case config.DbVersion == 0 || config.DbVersion > math.MaxInt64:
		return fmt.Errorf("%w: DbVersion must be between 1 and MaxInt64", ErrInvalidConfig)
	default:
		return nil
	}
}
