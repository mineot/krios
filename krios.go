// Package krios provides connection management, transactions, migrations and
// query building for SQLite3, MySQL and PostgreSQL on top of database/sql.
// Configuration, the connection lifecycle (Open and Close) and explicit
// transactions (Begin, Commit, Rollback with execution on DB and Tx) are
// implemented; migrations and query building are not implemented yet.
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

	// ErrTxDone indicates an operation attempted on a transaction that has
	// already been committed or rolled back.
	ErrTxDone = errors.New("krios: transaction already committed or rolled back")
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

func (db *DB) ensureOpen() error {
	db.mu.Lock()

	defer db.mu.Unlock()

	if db.closed {
		return ErrClosed
	}

	return nil
}

// Result exposes driver result information in a driver-agnostic way.
type Result interface {
	RowsAffected() (int64, bool)
	LastInsertId() (int64, bool)
}

type resultAdapter struct {
	r sql.Result
}

func (ra resultAdapter) RowsAffected() (int64, bool) {
	n, err := ra.r.RowsAffected()

	if err != nil {
		return 0, false
	}

	return n, true
}

func (ra resultAdapter) LastInsertId() (int64, bool) {
	id, err := ra.r.LastInsertId()

	if err != nil {
		return 0, false
	}

	return id, true
}

func wrapResult(r sql.Result) Result {
	if r == nil {
		return resultAdapter{}
	}

	return resultAdapter{r: r}
}

// Row is a wrapper around *sql.Row that preserves sql.ErrNoRows on Scan
// and does not expose the underlying raw pointer to the consumer.
type Row struct {
	r *sql.Row
}

// Scan copies the values from the matched row into the provided destinations.
// It returns sql.ErrNoRows if no row was found.
func (rw Row) Scan(dest ...any) error {
	if rw.r == nil {
		return sql.ErrNoRows
	}

	return rw.r.Scan(dest...)
}

func wrapRow(r *sql.Row) Row {
	return Row{r: r}
}

// Rows is an iterator over query results. The caller is responsible for
// calling Close(). Close() is idempotent. Iterating until Next() == false
// does not replace Close().
type Rows struct {
	r      *sql.Rows
	mu     sync.Mutex
	closed bool
}

// Next prepares the next row for reading.
func (rw *Rows) Next() bool {
	if rw == nil || rw.r == nil {
		return false
	}

	return rw.r.Next()
}

// Scan copies the values from the current row into the provided destinations.
func (rw *Rows) Scan(dest ...any) error {
	if rw == nil || rw.r == nil {
		return errors.New("krios: rows are closed or invalid")
	}

	return rw.r.Scan(dest...)
}

// Err returns the error, if any, that was encountered during iteration.
func (rw *Rows) Err() error {
	if rw == nil || rw.r == nil {
		return nil
	}

	return rw.r.Err()
}

// Close closes the underlying rows iterator. It is idempotent.
func (rw *Rows) Close() error {
	if rw == nil {
		return nil
	}

	rw.mu.Lock()

	defer rw.mu.Unlock()

	if rw.closed {
		return nil
	}

	rw.closed = true

	var err error

	if rw.r != nil {
		err = rw.r.Close()
	}

	rw.r = nil

	return err
}

func wrapRows(r *sql.Rows) *Rows {
	if r == nil {
		return nil
	}

	return &Rows{r: r}
}

// executor is the internal execution contract satisfied by the underlying
// *sql.DB and *sql.Tx. The public methods of DB and Tx validate their own
// state (open or tx done) and then delegate through these helpers, which
// check the context, call the underlying database/sql object and apply the
// public wrappers.
type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var (
	_ executor = (*sql.DB)(nil)
	_ executor = (*sql.Tx)(nil)
)

// execWith validates ctx, executes the query on ex and wraps the result.
// op identifies the operation in context errors ("exec" or "tx exec").
func execWith(ctx context.Context, ex executor, op, query string, args ...any) (Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: %s: %w", op, err)
	}

	res, err := ex.ExecContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	return wrapResult(res), nil
}

// queryWith validates ctx, executes the query on ex and wraps the rows.
// op identifies the operation in context errors ("query" or "tx query").
func queryWith(ctx context.Context, ex executor, op, query string, args ...any) (*Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: %s: %w", op, err)
	}

	r, err := ex.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	return wrapRows(r), nil
}

// queryRowWith validates ctx, executes the query on ex and wraps the row.
// A canceled or expired context yields a Row whose Scan reports sql.ErrNoRows.
func queryRowWith(ctx context.Context, ex executor, query string, args ...any) Row {
	if err := ctx.Err(); err != nil {
		return Row{r: nil}
	}

	return wrapRow(ex.QueryRowContext(ctx, query, args...))
}

// Tx represents an explicit transaction. Calls on the same Tx are serialized
// (not safe for concurrent use by multiple goroutines). Operations on Tx
// after Commit or Rollback return ErrTxDone. The first statement failure is
// remembered and blocks Commit: Commit rolls the transaction back and returns
// that error instead of confirming a partial group.
type Tx struct {
	tx   *sql.Tx
	mu   sync.Mutex // serialize execution calls on this transaction
	done bool
	err  error  // first statement failure; blocks Commit (see Commit)
	cfg  Config // optional: may be useful later (dialect); kept for consistency
}

// ExecContext executes a query without returning rows.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (Result, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	return execWith(ctx, db.sqlDB, "exec", query, args...)
}

// QueryContext executes a query that returns rows.
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	return queryWith(ctx, db.sqlDB, "query", query, args...)
}

// QueryRowContext executes a query that is expected to return at most one row.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	if err := db.ensureOpen(); err != nil {
		return Row{r: nil}
	}

	return queryRowWith(ctx, db.sqlDB, query, args...)
}

// Begin starts a transaction.
func (db *DB) Begin(ctx context.Context) (*Tx, error) {
	if err := db.ensureOpen(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: begin: %w", err)
	}

	stx, err := db.sqlDB.BeginTx(ctx, nil)

	if err != nil {
		return nil, err
	}

	return &Tx{
		tx:  stx,
		cfg: db.cfg,
	}, nil
}

// ExecContext executes a query without returning rows on this transaction.
// The first error is remembered and blocks Commit (see Commit).
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (Result, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return nil, ErrTxDone
	}

	res, err := execWith(ctx, tx.tx, "tx exec", query, args...)

	if err != nil && tx.err == nil {
		tx.err = err
	}

	return res, err
}

// QueryContext executes a query that returns rows on this transaction.
// The first error is remembered and blocks Commit (see Commit).
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return nil, ErrTxDone
	}

	r, err := queryWith(ctx, tx.tx, "tx query", query, args...)

	if err != nil && tx.err == nil {
		tx.err = err
	}

	return r, err
}

// QueryRowContext executes a query that is expected to return at most one row
// on this transaction.
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return Row{r: nil}
	}

	return queryRowWith(ctx, tx.tx, query, args...)
}

// Commit commits the transaction. If a previous ExecContext or QueryContext
// on this transaction failed, Commit does not confirm anything: it rolls the
// transaction back, marks it done and returns the original error (retrievable
// with errors.Is), so a partial group can never be confirmed. Subsequent
// calls then return ErrTxDone.
//
// Errors reported only after the fact — by Row.Scan or during Rows iteration
// from QueryRowContext — are not observable here and do not block Commit.
func (tx *Tx) Commit(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return ErrTxDone
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("krios: tx commit: %w", err)
	}

	if tx.err != nil {
		err := tx.err
		tx.err = nil

		if rbErr := tx.tx.Rollback(); rbErr != nil {
			tx.done = true
			return fmt.Errorf("krios: tx commit blocked by previous error: %w (rollback failed: %v)", err, rbErr)
		}

		tx.done = true
		return fmt.Errorf("krios: tx commit blocked by previous error: %w", err)
	}

	if err := tx.tx.Commit(); err != nil {
		return err
	}

	tx.done = true
	return nil
}

// Rollback rolls back the transaction.
func (tx *Tx) Rollback(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.done {
		return ErrTxDone
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("krios: tx rollback: %w", err)
	}

	if err := tx.tx.Rollback(); err != nil {
		return err
	}

	tx.done = true
	return nil
}

// buildDSN returns the driver-specific data source name.
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
