package core

import (
	"fmt"
	"time"
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

	// Port defaults to 3306 when zero; explicit values must be in 1..65535.
	Port int
}

// PostgresConfig holds the PostgreSQL connection settings.
type PostgresConfig struct {
	// Host, User, Password and DBName are required.
	Host     string
	User     string
	Password string
	DBName   string

	// Port defaults to 5432 when zero; explicit values must be in 1..65535.
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

	// Timeout is the time budget for opening the access, including preparation.
	// Interrupting a blocking operation requires a context-aware driver.
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
	case port < 0 || port > 65535:
		return fmt.Errorf("%w: %s port must be between 0 and 65535", ErrInvalidConfig, engine)
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
