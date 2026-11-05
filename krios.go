// Package krios provides the client configuration for the Krios SQL library.
package krios

import "github.com/mineot/krios/internal/core"

// Client holds the application's database configuration.
type Client = core.Client

// Config configures a client. The application owns its pool and context.
type Config = core.Config

// Dialect identifies the SQL dialect used by a client.
type Dialect = core.Dialect

// Migration describes the SQL files for a schema version.
type Migration = core.Migration

const (
	DialectSQLite3    = core.DialectSQLite3
	DialectPostgreSQL = core.DialectPostgreSQL
	DialectMySQL      = core.DialectMySQL
)

var (
	// ErrInvalidConfig identifies invalid client configuration.
	ErrInvalidConfig = core.ErrInvalidConfig
	// ErrInvalidQuery identifies invalid query structure.
	ErrInvalidQuery = core.ErrInvalidQuery
	// ErrInvalidMigration identifies invalid migration configuration or history.
	ErrInvalidMigration = core.ErrInvalidMigration
)

// New validates config and pings its pool. It does not run migrations or own
// the pool. On failure it returns a nil client and preserves the error cause.
func New(config Config) (*Client, error) {
	return core.New(config)
}
