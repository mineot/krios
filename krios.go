// Package krios provides connection management, transactions, migrations and
// query building for SQLite3, MySQL and PostgreSQL on top of database/sql.
// Configuration, the connection lifecycle (Open and Close), explicit
// transactions (Begin, Commit, Rollback with execution on DB and Tx) and
// reflection-based entity mapping with per-field validation and formatting are
// implemented; migrations and query building are not implemented yet.
//
// Drivers are chosen and imported by the consumer; Krios does not import them.
package krios

import (
	"context"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/transaction"
)

// Public aliases preserve the consumer API while implementation lives in core.
type (
	// DBType identifies a supported database engine.
	DBType = config.DBType
	// SQLiteConfig holds the SQLite connection settings.
	SQLiteConfig = config.SQLiteConfig
	// MySQLConfig holds the MySQL connection settings.
	MySQLConfig = config.MySQLConfig
	// PostgresConfig holds the PostgreSQL connection settings.
	PostgresConfig = config.PostgresConfig
	// MigrationsConfig reserves future migration settings.
	MigrationsConfig = config.MigrationsConfig
	// Config describes database access and is passed to Open.
	Config = config.Config
	// DB is the database access returned by Open. It supports concurrent use.
	DB = access.DB
	// Tx is an explicit transaction returned by DB.Begin.
	Tx = transaction.Tx
	// Result exposes affected rows and generated IDs with availability flags.
	Result = result.Result
	// Row defers a single-row query's result or error until Scan.
	Row = scan.Row
	// Rows is a query iterator. The consumer must close it and check Err.
	Rows = scan.Rows
	// Entity is the descriptor derived by reflection from a Go struct type.
	Entity = entity.Entity
	// Validator is implemented by field types that validate their own value.
	Validator = entity.Validator
	// Formatter is implemented by field types that provide a presentation text.
	Formatter = entity.Formatter
)

const (
	DBTypeSQLite   = config.DBTypeSQLite
	DBTypeMySQL    = config.DBTypeMySQL
	DBTypePostgres = config.DBTypePostgres
)

var (
	// ErrInvalidConfig indicates missing or inconsistent configuration values.
	ErrInvalidConfig = errs.ErrInvalidConfig
	// ErrDriverNotLoaded indicates an unregistered driver name.
	ErrDriverNotLoaded = errs.ErrDriverNotLoaded
	// ErrConnectionFailed identifies a connection failure, preserving its cause.
	ErrConnectionFailed = errs.ErrConnectionFailed
	// ErrClosed indicates an operation attempted after DB.Close.
	ErrClosed = errs.ErrClosed
	// ErrTxDone indicates an operation attempted on a terminal transaction.
	ErrTxDone = errs.ErrTxDone
	// ErrInvalidEntity indicates that a struct cannot be mapped into an entity.
	ErrInvalidEntity = errs.ErrInvalidEntity
	// ErrValidation indicates that a field rejected its own value.
	ErrValidation = errs.ErrValidation
	// ErrFormat indicates that a field failed to produce its presentation text.
	ErrFormat = errs.ErrFormat
)

// Open validates cfg, opens database access and verifies connectivity.
// The effective deadline is the shorter of ctx's deadline and cfg.Timeout
// (10 seconds when zero). Blocking calls require driver cooperation with
// cancellation. Resources created during a failed attempt are released.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	return access.Open(ctx, cfg)
}

// EntityOf derives and returns the entity descriptor for T, which may be a
// struct type or a pointer to one. Table and columns come from the type name
// (snake_case, overridable with TableName() string) and the krios field tags.
// The descriptor is cached per type. Invalid entities return ErrInvalidEntity.
//
// A field tag has the form `krios:"column[,pk]"`; `krios:"-"` ignores the
// field and a missing or empty tag derives the column from the field name.
func EntityOf[T any]() (*Entity, error) {
	return entity.EntityOf[T]()
}
