package tests

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

func TestOpenMySQLWithFailingDriverReturnsConnectionFailed(t *testing.T) {
	t.Parallel()

	cfg := krios.Config{
		Type:   krios.DBTypeMySQL,
		Driver: "krios_fail",
		MySQL: &krios.MySQLConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
			Port:     0, // default 3306
		},
		Timeout: time.Second,
	}

	db, err := krios.Open(context.Background(), cfg)
	if db != nil {
		_ = db.Close()
		t.Fatal("expected db to be nil")
	}
	if !errors.Is(err, krios.ErrConnectionFailed) {
		t.Fatalf("Open(MySQL) = %v, want ErrConnectionFailed", err)
	}
}

func TestOpenPostgresWithFailingDriverReturnsConnectionFailed(t *testing.T) {
	t.Parallel()

	cfg := krios.Config{
		Type:   krios.DBTypePostgres,
		Driver: "krios_fail",
		Postgres: &krios.PostgresConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
			Port:     0, // default 5432
		},
		Timeout: time.Second,
	}

	db, err := krios.Open(context.Background(), cfg)
	if db != nil {
		_ = db.Close()
		t.Fatal("expected db to be nil")
	}
	if !errors.Is(err, krios.ErrConnectionFailed) {
		t.Fatalf("Open(Postgres) = %v, want ErrConnectionFailed", err)
	}
}

func TestOpenMySQLInvalidConfigReturnsInvalidConfig(t *testing.T) {
	t.Parallel()

	cases := []func() krios.Config{
		func() krios.Config {
			cfg := baseMySQL()
			cfg.MySQL.Host = ""
			return cfg
		},
		func() krios.Config {
			cfg := baseMySQL()
			cfg.MySQL.User = ""
			return cfg
		},
		func() krios.Config {
			cfg := baseMySQL()
			cfg.MySQL.Password = ""
			return cfg
		},
		func() krios.Config {
			cfg := baseMySQL()
			cfg.MySQL.DBName = ""
			return cfg
		},
		func() krios.Config {
			cfg := baseMySQL()
			cfg.MySQL.Port = -1
			return cfg
		},
		func() krios.Config {
			cfg := baseMySQL()
			cfg.MySQL = nil
			return cfg
		},
		func() krios.Config {
			cfg := baseMySQL()
			cfg.Postgres = &krios.PostgresConfig{Host: "h", User: "u", Password: "p", DBName: "d"}
			return cfg
		},
	}

	for i, fn := range cases {
		cfg := fn()
		if _, err := krios.Open(context.Background(), cfg); !errors.Is(err, krios.ErrInvalidConfig) {
			t.Errorf("case %d Open(MySQL invalid) = %v, want ErrInvalidConfig", i, err)
		}
	}
}

func TestOpenPostgresInvalidConfigReturnsInvalidConfig(t *testing.T) {
	t.Parallel()

	cases := []func() krios.Config{
		func() krios.Config {
			cfg := basePostgres()
			cfg.Postgres.Host = ""
			return cfg
		},
		func() krios.Config {
			cfg := basePostgres()
			cfg.Postgres.Password = ""
			return cfg
		},
		func() krios.Config {
			cfg := basePostgres()
			cfg.Postgres.Port = -1
			return cfg
		},
		func() krios.Config {
			cfg := basePostgres()
			cfg.Postgres = nil
			return cfg
		},
	}

	for i, fn := range cases {
		cfg := fn()
		if _, err := krios.Open(context.Background(), cfg); !errors.Is(err, krios.ErrInvalidConfig) {
			t.Errorf("case %d Open(Postgres invalid) = %v, want ErrInvalidConfig", i, err)
		}
	}
}

func TestOpenSQLiteRequiresNoMigrations(t *testing.T) {
	t.Parallel()

	cfg := krios.Config{
		Type:   krios.DBTypeSQLite,
		Driver: "sqlite",
		SQLite: &krios.SQLiteConfig{DBPath: filepath.Join(t.TempDir(), "nomig.db")},
		// Migrations intentionally nil
	}

	db, err := krios.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open(SQLite, no migrations) = %v, want nil", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
}

func baseMySQL() krios.Config {
	return krios.Config{
		Type:   krios.DBTypeMySQL,
		Driver: "krios_fail",
		MySQL: &krios.MySQLConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
		},
	}
}

func basePostgres() krios.Config {
	return krios.Config{
		Type:   krios.DBTypePostgres,
		Driver: "krios_fail",
		Postgres: &krios.PostgresConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
		},
	}
}
