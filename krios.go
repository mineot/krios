// Package krios provides connection management, transactions, migrations and
// query building for SQLite3, MySQL and PostgreSQL on top of database/sql.
// Configuration and the connection lifecycle (Open and Close) are implemented;
// transactions, migrations and query building are not implemented yet.
//
// Drivers are chosen and imported by the consumer; Krios does not import them.
package krios

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"
)

var (
	// ErrInvalidConfig indicates an invalid configuration: missing required
	// fields, inconsistent values or a negative Timeout. The message details
	// the offending field.
	ErrInvalidConfig = errors.New("krios: invalid configuration")

	// ErrDriverNotLoaded indicates that the driver named in Config.Driver is
	// not registered in database/sql: the consumer did not import the driver
	// or registered it under a different name.
	ErrDriverNotLoaded = errors.New("krios: database driver not loaded for the configured type")

	// ErrConnectionFailed indicates a failure to connect (network, file,
	// permission, database refusal). It wraps the original driver error.
	ErrConnectionFailed = errors.New("krios: failed to connect to database")

	// ErrClosed indicates an operation attempted after Close.
	ErrClosed = errors.New("krios: database access is closed")
)

const (
	defaultTimeout      = 10 * time.Second
	defaultMySQLPort    = 3306
	defaultPostgresPort = 5432
)

// DBType identifies one of the databases supported by Krios.
type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypeMySQL    DBType = "mysql"
	DBTypePostgres DBType = "postgres"
)

// SQLiteConfig holds the SQLite3 connection settings.
type SQLiteConfig struct {
	// DBPath is the absolute or relative path of the .db file.
	// Example: "dev.db", "/data/app.db". Required.
	DBPath string
}

// MySQLConfig holds the MySQL connection settings.
type MySQLConfig struct {
	// Host, User, Password and DBName are required.
	Host     string
	User     string
	Password string
	DBName   string

	// Port defaults to 3306 when zero; negative values are invalid.
	Port int
}

// PostgresConfig holds the PostgreSQL connection settings.
type PostgresConfig struct {
	// Host, User, Password and DBName are required.
	Host     string
	User     string
	Password string
	DBName   string

	// Port defaults to 5432 when zero; negative values are invalid.
	Port int
}

// MigrationsConfig is reserved for the requested version and the association
// of migration files. Its contents will be defined in E08-P01.
type MigrationsConfig struct{}

// Config describes the database access. It is filled in code by the consumer
// and passed to Open, with no external configuration file required.
type Config struct {
	// Type selects the database. Required.
	Type DBType

	// Driver is the name under which the driver imported by the consumer is
	// registered in database/sql, such as "sqlite", "mysql" or "pgx".
	// Required and must match the imported (blank) driver.
	Driver string

	// SQLite is required when Type is DBTypeSQLite.
	SQLite *SQLiteConfig

	// MySQL is required when Type is DBTypeMySQL.
	MySQL *MySQLConfig

	// Postgres is required when Type is DBTypePostgres.
	Postgres *PostgresConfig

	// Timeout bounds how long Open waits to open the connection.
	// Zero applies the default of 10 seconds; negative values are invalid.
	Timeout time.Duration

	// Migrations is optional. It may be nil when migrations are not used.
	Migrations *MigrationsConfig
}

func (cfg Config) validate() error {
	switch cfg.Type {
	case DBTypeSQLite, DBTypeMySQL, DBTypePostgres:
	default:
		return fmt.Errorf("%w: unsupported database type %q", ErrInvalidConfig, cfg.Type)
	}

	if cfg.Driver == "" {
		return fmt.Errorf("%w: driver name is required", ErrInvalidConfig)
	}

	switch cfg.Type {
	case DBTypeSQLite:
		if cfg.SQLite == nil {
			return fmt.Errorf("%w: SQLite configuration is required for type %q", ErrInvalidConfig, cfg.Type)
		}
	case DBTypeMySQL:
		if cfg.MySQL == nil {
			return fmt.Errorf("%w: MySQL configuration is required for type %q", ErrInvalidConfig, cfg.Type)
		}
	case DBTypePostgres:
		if cfg.Postgres == nil {
			return fmt.Errorf("%w: PostgreSQL configuration is required for type %q", ErrInvalidConfig, cfg.Type)
		}
	}

	if cfg.SQLite != nil && cfg.Type != DBTypeSQLite {
		return fmt.Errorf("%w: SQLite configuration set for type %q", ErrInvalidConfig, cfg.Type)
	}

	if cfg.MySQL != nil && cfg.Type != DBTypeMySQL {
		return fmt.Errorf("%w: MySQL configuration set for type %q", ErrInvalidConfig, cfg.Type)
	}

	if cfg.Postgres != nil && cfg.Type != DBTypePostgres {
		return fmt.Errorf("%w: PostgreSQL configuration set for type %q", ErrInvalidConfig, cfg.Type)
	}

	switch cfg.Type {
	case DBTypeSQLite:
		if cfg.SQLite.DBPath == "" {
			return fmt.Errorf("%w: SQLite DBPath is required", ErrInvalidConfig)
		}
	case DBTypeMySQL:
		if err := validateConnection("MySQL", cfg.MySQL.Host, cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.DBName, cfg.MySQL.Port); err != nil {
			return err
		}
	case DBTypePostgres:
		if err := validateConnection("PostgreSQL", cfg.Postgres.Host, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.DBName, cfg.Postgres.Port); err != nil {
			return err
		}
	}

	if cfg.Timeout < 0 {
		return fmt.Errorf("%w: timeout must not be negative", ErrInvalidConfig)
	}

	return nil
}

func validateConnection(engine, host, user, password, database string, port int) error {
	switch {
	case host == "":
		return fmt.Errorf("%w: %s host is required", ErrInvalidConfig, engine)
	case user == "":
		return fmt.Errorf("%w: %s user is required", ErrInvalidConfig, engine)
	case password == "":
		return fmt.Errorf("%w: %s password is required", ErrInvalidConfig, engine)
	case database == "":
		return fmt.Errorf("%w: %s database name is required", ErrInvalidConfig, engine)
	case port < 0:
		return fmt.Errorf("%w: %s port must not be negative", ErrInvalidConfig, engine)
	}
	return nil
}

func (cfg Config) timeout() time.Duration {
	if cfg.Timeout == 0 {
		return defaultTimeout
	}

	return cfg.Timeout
}

func (c MySQLConfig) port() int {
	if c.Port == 0 {
		return defaultMySQLPort
	}

	return c.Port
}

func (c PostgresConfig) port() int {
	if c.Port == 0 {
		return defaultPostgresPort
	}

	return c.Port
}

// DB is the database access opened by Open. It is safe for concurrent use.
type DB struct {
	sqlDB *sql.DB
	cfg   Config

	mu     sync.Mutex
	closed bool
}

// Open validates cfg, checks that the configured driver is registered in
// database/sql, opens the access and verifies connectivity.
//
// The effective deadline is the shortest of the context deadline and
// cfg.Timeout (default 10 seconds when zero). Cancellation and deadline are
// reported as wrapped context.Canceled or context.DeadlineExceeded, without
// ErrConnectionFailed. Any resource created is released when opening fails.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if !slices.Contains(sql.Drivers(), cfg.Driver) {
		return nil, fmt.Errorf("%w: %q", ErrDriverNotLoaded, cfg.Driver)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: open: %w", err)
	}

	sqlDB, err := sql.Open(cfg.Driver, buildDSN(cfg))

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.timeout())

	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()

		if ctxErr := pingCtx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("krios: open: %w", ctxErr)
		}

		return nil, fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	if err := pingCtx.Err(); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf("krios: open: %w", err)
	}

	return &DB{sqlDB: sqlDB, cfg: cfg}, nil
}

// Close releases the resources associated with the access. It is idempotent:
// after the first call, subsequent calls return nil.
func (db *DB) Close() error {
	db.mu.Lock()

	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	db.closed = true

	return db.sqlDB.Close()
}

// ensureOpen reports ErrClosed after Close. Operations use it before touching
// the database.
func (db *DB) ensureOpen() error {
	db.mu.Lock()

	defer db.mu.Unlock()

	if db.closed {
		return ErrClosed
	}

	return nil
}

func buildDSN(cfg Config) string {
	switch cfg.Type {
	case DBTypeSQLite:
		return cfg.SQLite.DBPath
	case DBTypeMySQL:
		m := cfg.MySQL
		return fmt.Sprintf("%s:%s@tcp(%s)/%s", m.User, m.Password, net.JoinHostPort(m.Host, strconv.Itoa(m.port())), m.DBName)
	case DBTypePostgres:
		p := cfg.Postgres
		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(p.User, p.Password),
			Host:   net.JoinHostPort(p.Host, strconv.Itoa(p.port())),
			Path:   "/" + p.DBName,
		}
		return u.String()
	default:
		return ""
	}
}
