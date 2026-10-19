package examples

import (
	"context"
	"errors"
	"time"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite" // registered under the name "sqlite"
)

// SQLiteConfig returns the configuration used by Run: one local SQLite file
// with a five second budget for opening the access.
func SQLiteConfig(dbPath string) krios.Config {
	return krios.Config{
		Type:    krios.DBTypeSQLite,
		Driver:  "sqlite",
		SQLite:  &krios.SQLiteConfig{DBPath: dbPath},
		Timeout: 5 * time.Second,
	}
}

// MySQLConfig returns the equivalent configuration for MySQL. The consumer
// registers the driver with _ "github.com/go-sql-driver/mysql"; Port zero
// applies the MySQL default of 3306.
func MySQLConfig(host, user, password, dbName string) krios.Config {
	return krios.Config{
		Type:   krios.DBTypeMySQL,
		Driver: "mysql",
		MySQL: &krios.MySQLConfig{
			Host:     host,
			User:     user,
			Password: password,
			DBName:   dbName,
		},
		Timeout: 5 * time.Second,
	}
}

// PostgresConfig returns the equivalent configuration for PostgreSQL. The
// consumer registers the driver with _ "github.com/jackc/pgx/v5/stdlib";
// Port zero applies the PostgreSQL default of 5432.
func PostgresConfig(host, user, password, dbName string) krios.Config {
	return krios.Config{
		Type:   krios.DBTypePostgres,
		Driver: "pgx",
		Postgres: &krios.PostgresConfig{
			Host:     host,
			User:     user,
			Password: password,
			DBName:   dbName,
		},
		Timeout: 5 * time.Second,
	}
}

// OpenStore validates cfg, opens the database access and wraps it in a Store.
// Failures match the krios sentinel errors; see OpenErrorKind.
func OpenStore(ctx context.Context, cfg krios.Config) (*Store, error) {
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// OpenErrorKind classifies a failed open for the caller's message. Consumers
// branch with errors.Is on the krios sentinel errors instead of parsing text.
func OpenErrorKind(err error) string {
	switch {
	case errors.Is(err, krios.ErrInvalidConfig):
		return "configuração inválida"
	case errors.Is(err, krios.ErrDriverNotLoaded):
		return "driver não importado"
	case errors.Is(err, krios.ErrConnectionFailed):
		return "falha de conexão"
	default:
		return "erro inesperado"
	}
}
