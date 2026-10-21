// Package config defines the database configuration, validates it and builds
// the driver-specific data source names.
package config

import (
	"fmt"
	"io/fs"
	"time"

	"github.com/mineot/krios/internal/errs"
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

// MigrationVersion associates a schema version with the embedded files that
// implement it in each direction (E08-P01 §4). The numeric prefix of a file
// name is only a naming convention: the version and order come from this
// declaration.
type MigrationVersion struct {
	// Version is the version number. Required, unique and > 0.
	Version int

	// Up lists the up files in execution order.
	Up []string

	// Down lists the down files in execution order.
	Down []string
}

// MigrationsConfig configures versioned migrations. It is filled in code and
// optional: when Config.Migrations is nil the library never loads or runs
// migrations. The versioned files are consumer-embedded (go:embed) and read
// through NewSQLFile (E08-P01 §4).
type MigrationsConfig struct {
	// FS receives the consumer's embedded resources (for example an embed.FS).
	FS fs.FS

	// TargetVersion is the desired version. Always explicit (pointer); nil is
	// invalid, &0 requests reverting all applied migrations.
	TargetVersion *int

	// Versions associates versions with files. The declared order defines the
	// sequence between versions.
	Versions []MigrationVersion

	// ManagedObjects lists the objects (tables) under Krios management, used
	// to verify a database without history (E08-P01 §6).
	ManagedObjects []string
}

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

// Validate reports whether the configuration is complete and consistent.
// Failures wrap ErrInvalidConfig.
func (cfg Config) Validate() error {
	switch cfg.Type {
	case DBTypeSQLite, DBTypeMySQL, DBTypePostgres:
	default:
		return fmt.Errorf("%w: unsupported database type %q", errs.ErrInvalidConfig, cfg.Type)
	}

	if cfg.Driver == "" {
		return fmt.Errorf("%w: driver name is required", errs.ErrInvalidConfig)
	}

	switch cfg.Type {
	case DBTypeSQLite:
		if cfg.SQLite == nil {
			return fmt.Errorf("%w: SQLite configuration is required for type %q", errs.ErrInvalidConfig, cfg.Type)
		}
	case DBTypeMySQL:
		if cfg.MySQL == nil {
			return fmt.Errorf("%w: MySQL configuration is required for type %q", errs.ErrInvalidConfig, cfg.Type)
		}
	case DBTypePostgres:
		if cfg.Postgres == nil {
			return fmt.Errorf("%w: PostgreSQL configuration is required for type %q", errs.ErrInvalidConfig, cfg.Type)
		}
	}

	if cfg.SQLite != nil && cfg.Type != DBTypeSQLite {
		return fmt.Errorf("%w: SQLite configuration set for type %q", errs.ErrInvalidConfig, cfg.Type)
	}

	if cfg.MySQL != nil && cfg.Type != DBTypeMySQL {
		return fmt.Errorf("%w: MySQL configuration set for type %q", errs.ErrInvalidConfig, cfg.Type)
	}

	if cfg.Postgres != nil && cfg.Type != DBTypePostgres {
		return fmt.Errorf("%w: PostgreSQL configuration set for type %q", errs.ErrInvalidConfig, cfg.Type)
	}

	switch cfg.Type {
	case DBTypeSQLite:
		if cfg.SQLite.DBPath == "" {
			return fmt.Errorf("%w: SQLite DBPath is required", errs.ErrInvalidConfig)
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
		return fmt.Errorf("%w: timeout must not be negative", errs.ErrInvalidConfig)
	}

	if cfg.Migrations != nil {
		if err := cfg.Migrations.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate reports whether the migration configuration is complete and
// consistent. Failures wrap ErrInvalidConfig. The checks are structural: the
// embedded files themselves are loaded by the migration preparation layer.
func (m MigrationsConfig) Validate() error {
	if m.FS == nil {
		return fmt.Errorf("%w: migrations require an FS with embedded resources", errs.ErrInvalidConfig)
	}

	if len(m.ManagedObjects) == 0 {
		return fmt.Errorf("%w: migrations require at least one managed object", errs.ErrInvalidConfig)
	}

	if len(m.Versions) == 0 {
		return fmt.Errorf("%w: migrations require at least one version", errs.ErrInvalidConfig)
	}

	if m.TargetVersion == nil {
		return fmt.Errorf("%w: migrations require TargetVersion", errs.ErrInvalidConfig)
	}

	if *m.TargetVersion < 0 {
		return fmt.Errorf("%w: migration target version must be >= 0", errs.ErrInvalidConfig)
	}

	seen := make(map[int]bool, len(m.Versions))
	previous := -1

	for _, v := range m.Versions {
		if v.Version <= 0 {
			return fmt.Errorf("%w: migration version must be greater than zero", errs.ErrInvalidConfig)
		}

		if seen[v.Version] {
			return fmt.Errorf("%w: duplicate migration version %d", errs.ErrInvalidConfig, v.Version)
		}

		if v.Version <= previous {
			return fmt.Errorf("%w: migration versions must be declared in strictly increasing order", errs.ErrInvalidConfig)
		}

		if err := checkDirection("up", v.Up); err != nil {
			return err
		}

		if err := checkDirection("down", v.Down); err != nil {
			return err
		}

		seen[v.Version] = true
		previous = v.Version
	}

	return nil
}

// checkDirection rejects blank and duplicated file names within one direction
// of a version.
func checkDirection(direction string, files []string) error {
	seen := make(map[string]bool, len(files))

	for _, name := range files {
		if name == "" {
			return fmt.Errorf("%w: %s migration contains a blank file name", errs.ErrInvalidConfig, direction)
		}

		if seen[name] {
			return fmt.Errorf("%w: duplicate %s migration file %q", errs.ErrInvalidConfig, direction, name)
		}

		seen[name] = true
	}

	return nil
}

func validateConnection(engine, host, user, password, database string, port int) error {
	switch {
	case host == "":
		return fmt.Errorf("%w: %s host is required", errs.ErrInvalidConfig, engine)
	case user == "":
		return fmt.Errorf("%w: %s user is required", errs.ErrInvalidConfig, engine)
	case password == "":
		return fmt.Errorf("%w: %s password is required", errs.ErrInvalidConfig, engine)
	case database == "":
		return fmt.Errorf("%w: %s database name is required", errs.ErrInvalidConfig, engine)
	case port < 0 || port > 65535:
		return fmt.Errorf("%w: %s port must be between 0 and 65535", errs.ErrInvalidConfig, engine)
	}
	return nil
}

// EffectiveTimeout returns the effective time budget, applying the default
// when the configured value is zero.
func (cfg Config) EffectiveTimeout() time.Duration {
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
