// Package krios provides connection management, transactions, migrations and
// query building for SQLite3, MySQL and PostgreSQL on top of database/sql.
// Configuration, the connection lifecycle (Open and Close), explicit
// transactions (Begin, Commit, Rollback with execution on DB and Tx) and
// reflection-based entity mapping are implemented; migrations and query
// building are not implemented yet.
//
// Drivers are chosen and imported by the consumer; Krios does not import them.
package krios

import (
	"context"

	"github.com/mineot/krios/internal/core"
)

// Public aliases preserve the consumer API while implementation lives in core.
type (
	// DBType identifies a supported database engine.
	DBType = core.DBType
	// SQLiteConfig holds the SQLite connection settings.
	SQLiteConfig = core.SQLiteConfig
	// MySQLConfig holds the MySQL connection settings.
	MySQLConfig = core.MySQLConfig
	// PostgresConfig holds the PostgreSQL connection settings.
	PostgresConfig = core.PostgresConfig
	// MigrationsConfig reserves future migration settings.
	MigrationsConfig = core.MigrationsConfig
	// Config describes database access and is passed to Open.
	Config = core.Config
	// DB is the database access returned by Open. It supports concurrent use.
	DB = core.DB
	// Tx is an explicit transaction returned by DB.Begin.
	Tx = core.Tx
	// Result exposes affected rows and generated IDs with availability flags.
	Result = core.Result
	// Row defers a single-row query's result or error until Scan.
	Row = core.Row
	// Rows is a query iterator. The consumer must close it and check Err.
	Rows = core.Rows
	// Entity is the descriptor derived by reflection from a Go struct type.
	Entity = core.Entity
)

const (
	DBTypeSQLite   = core.DBTypeSQLite
	DBTypeMySQL    = core.DBTypeMySQL
	DBTypePostgres = core.DBTypePostgres
)

var (
	// ErrInvalidConfig indicates missing or inconsistent configuration values.
	ErrInvalidConfig = core.ErrInvalidConfig
	// ErrDriverNotLoaded indicates an unregistered driver name.
	ErrDriverNotLoaded = core.ErrDriverNotLoaded
	// ErrConnectionFailed identifies a connection failure, preserving its cause.
	ErrConnectionFailed = core.ErrConnectionFailed
	// ErrClosed indicates an operation attempted after DB.Close.
	ErrClosed = core.ErrClosed
	// ErrTxDone indicates an operation attempted on a terminal transaction.
	ErrTxDone = core.ErrTxDone
	// ErrInvalidEntity indicates that a struct cannot be mapped into an entity.
	ErrInvalidEntity = core.ErrInvalidEntity
)

// Open validates cfg, opens database access and verifies connectivity.
// The effective deadline is the shorter of ctx's deadline and cfg.Timeout
// (10 seconds when zero). Blocking calls require driver cooperation with
// cancellation. Resources created during a failed attempt are released.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	return core.Open(ctx, cfg)
}

// EntityOf derives and returns the entity descriptor for T, which may be a
// struct type or a pointer to one. Table and columns come from the type name
// (snake_case, overridable with TableName() string) and the krios field tags.
// The descriptor is cached per type. Invalid entities return ErrInvalidEntity.
//
// A field tag has the form `krios:"column[,pk]"`; `krios:"-"` ignores the
// field and a missing or empty tag derives the column from the field name.
func EntityOf[T any]() (*Entity, error) {
	return core.EntityOf[T]()
}
