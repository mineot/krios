// Package krios provides connection management, transactions, migrations,
// query building and entity-scoped writes for SQLite3, MySQL and PostgreSQL on
// top of database/sql. Configuration, the connection lifecycle (Open and
// Close), explicit transactions (Begin, Commit, Rollback with execution on DB
// and Tx), reflection-based entity mapping with per-field validation and
// formatting, chained SELECT building with filters, joins, aggregates,
// ordering and limit/offset pagination, chained INSERT/UPDATE/DELETE with
// the value policy, mandatory filters and per-database RETURNING support,
// execution of consumer-embedded SQL files (NewSQLFile with DB.File or
// Tx.File) and versioned migrations (RunMigrations, which loads resources,
// inspects history, builds the ordered plan and applies it with the maximum
// viable transactional scope) are implemented.
//
// Drivers are chosen and imported by the consumer; Krios does not import them.
package krios

import (
	"context"
	"io/fs"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/migrate"
	"github.com/mineot/krios/internal/query"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/sqlfile"
	"github.com/mineot/krios/internal/transaction"
	"github.com/mineot/krios/internal/write"
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
	// MigrationsConfig configures versioned migrations: the embedded
	// resources, the desired target version, the version/file associations
	// and the managed objects. It is optional in Config.
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
	// Column references a mapped column and builds query conditions.
	Column = entity.Column
	// Condition is an opaque boolean predicate used by Where and Join.
	Condition = entity.Condition
	// LikeMode selects how Column.Like surrounds its value with wildcards.
	LikeMode = entity.LikeMode
	// Select is the chained SELECT builder created by DB.Select or Tx.Select.
	Select = query.Select
	// Insert is the chained INSERT builder created by DB.Insert or Tx.Insert.
	Insert = write.Insert
	// Update is the chained UPDATE builder created by DB.Update or Tx.Update.
	Update = write.Update
	// Delete is the chained DELETE builder created by DB.Delete or Tx.Delete.
	Delete = write.Delete
	// Expr is a selectable expression: a Column or an Aggregate.
	Expr = entity.Expr
	// Aggregate is an aggregate expression such as COUNT or SUM.
	Aggregate = entity.Aggregate
	// Sort is one ORDER BY criterion built with Asc or Desc.
	Sort = entity.Sort
	// Paginate describes one page returned by Select.Paginate.
	Paginate = query.Paginate
	// JoinKind selects the join operation of Select.Join.
	JoinKind = query.JoinKind
	// Validator is implemented by field types that validate their own value.
	Validator = entity.Validator
	// Formatter is implemented by field types that provide a presentation text.
	Formatter = entity.Formatter
	// SQLFile is the immutable reference to a consumer-embedded SQL resource.
	SQLFile = sqlfile.SQLFile
	// File is the runner created by DB.File or Tx.File that executes a SQLFile.
	File = sqlfile.File
	// MigrationVersion associates a schema version with its up and down files.
	MigrationVersion = config.MigrationVersion
	// MigrationPlan is the ordered set of steps prepared by RunMigrations.
	MigrationPlan = migrate.Plan
	// MigrationStep is one file to execute within a version and direction.
	MigrationStep = migrate.Step
	// PartialMigrationError carries the context of a blocked migration run.
	// Use errors.As to recover it from an ErrPartialMigration.
	PartialMigrationError = errs.PartialMigrationError
)

const (
	DBTypeSQLite   = config.DBTypeSQLite
	DBTypeMySQL    = config.DBTypeMySQL
	DBTypePostgres = config.DBTypePostgres
)

// LIKE matching modes accepted by Column.Like.
const (
	Contains   = entity.Contains
	StartsWith = entity.StartsWith
	EndsWith   = entity.EndsWith
)

// Join kinds accepted by Select.Join.
const (
	InnerJoin = query.InnerJoin
	LeftJoin  = query.LeftJoin
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
	// ErrInvalidQuery indicates an invalid query construction, reported before
	// execution.
	ErrInvalidQuery = errs.ErrInvalidQuery
	// ErrFileNotFound indicates that an embedded SQL resource could not be
	// loaded (nil file system, invalid or missing path, directory or blank
	// file).
	ErrFileNotFound = errs.ErrFileNotFound
	// ErrPartialMigration indicates that a migration run is blocked by an
	// in_progress or failed record, or by managed objects in a database
	// without history. Use errors.As with *PartialMigrationError for context.
	ErrPartialMigration = errs.ErrPartialMigration
)

// Open validates cfg, opens database access and verifies connectivity.
// The effective deadline is the shorter of ctx's deadline and cfg.Timeout
// (10 seconds when zero). Blocking calls require driver cooperation with
// cancellation. Resources created during a failed attempt are released.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	return access.Open(ctx, cfg)
}

// NewSQLFile loads path from the consumer's fs.FS (for example an embed.FS)
// and returns the immutable embedded SQL resource. Loading is eager; a nil
// file system, an invalid or missing path, a directory or a blank file fail
// with ErrFileNotFound before any database access.
//
// The resource is executed outside a transaction with DB.File or inside one
// with Tx.File. Parameters are supplied separately, using the markers the
// configured driver accepts, and no syntax translation happens.
func NewSQLFile(fsys fs.FS, path string) (*SQLFile, error) {
	return sqlfile.NewSQLFile(fsys, path)
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

// And combines conditions into a conjunction. An empty group is invalid and
// fails rendering with ErrInvalidQuery.
func And(conditions ...Condition) Condition {
	return entity.And(conditions...)
}

// Or combines conditions into a disjunction, nestable with And and Or. An
// empty group is invalid and fails rendering with ErrInvalidQuery.
func Or(conditions ...Condition) Condition {
	return entity.Or(conditions...)
}

// Count returns the COUNT(*) aggregate, used in Select.Fields to count rows.
// Use Column.Count for COUNT(column), which skips NULL values.
func Count() Aggregate {
	return entity.Count()
}
