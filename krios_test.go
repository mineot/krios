package krios

import (
	"errors"
	"testing"
	"time"
)

func validSQLite() Config {
	return Config{
		Type:   DBTypeSQLite,
		Driver: "sqlite",
		SQLite: &SQLiteConfig{DBPath: "dev.db"},
	}
}

func validMySQL() Config {
	return Config{
		Type:   DBTypeMySQL,
		Driver: "mysql",
		MySQL: &MySQLConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
		},
	}
}

func validPostgres() Config {
	return Config{
		Type:   DBTypePostgres,
		Driver: "pgx",
		Postgres: &PostgresConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
		},
	}
}

func TestConfigValidateAcceptsValidConfigurations(t *testing.T) {
	t.Parallel()

	for _, cfg := range []Config{validSQLite(), validMySQL(), validPostgres()} {
		if err := cfg.validate(); err != nil {
			t.Errorf("validate(%q) = %v, want nil", cfg.Type, err)
		}
	}
}

func TestConfigValidateRejectsInvalidConfigurations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  func() Config
	}{
		{
			name: "unknown type",
			cfg: func() Config {
				cfg := validSQLite()
				cfg.Type = "oracle"
				return cfg
			},
		},
		{
			name: "empty driver",
			cfg: func() Config {
				cfg := validSQLite()
				cfg.Driver = ""
				return cfg
			},
		},
		{
			name: "missing sqlite configuration",
			cfg: func() Config {
				cfg := validSQLite()
				cfg.SQLite = nil
				return cfg
			},
		},
		{
			name: "missing mysql configuration",
			cfg: func() Config {
				cfg := validMySQL()
				cfg.MySQL = nil
				return cfg
			},
		},
		{
			name: "missing postgres configuration",
			cfg: func() Config {
				cfg := validPostgres()
				cfg.Postgres = nil
				return cfg
			},
		},
		{
			name: "outside configuration set",
			cfg: func() Config {
				cfg := validSQLite()
				cfg.MySQL = &MySQLConfig{Host: "h", User: "u", Password: "p", DBName: "d"}
				return cfg
			},
		},
		{
			name: "empty sqlite path",
			cfg: func() Config {
				cfg := validSQLite()
				cfg.SQLite.DBPath = ""
				return cfg
			},
		},
		{
			name: "empty mysql host",
			cfg: func() Config {
				cfg := validMySQL()
				cfg.MySQL.Host = ""
				return cfg
			},
		},
		{
			name: "empty mysql password",
			cfg: func() Config {
				cfg := validMySQL()
				cfg.MySQL.Password = ""
				return cfg
			},
		},
		{
			name: "negative mysql port",
			cfg: func() Config {
				cfg := validMySQL()
				cfg.MySQL.Port = -1
				return cfg
			},
		},
		{
			name: "empty postgres host",
			cfg: func() Config {
				cfg := validPostgres()
				cfg.Postgres.Host = ""
				return cfg
			},
		},
		{
			name: "negative postgres port",
			cfg: func() Config {
				cfg := validPostgres()
				cfg.Postgres.Port = -1
				return cfg
			},
		},
		{
			name: "negative timeout",
			cfg: func() Config {
				cfg := validSQLite()
				cfg.Timeout = -time.Second
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.cfg().validate(); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("validate() = %v, want error wrapping ErrInvalidConfig", err)
			}
		})
	}
}

func TestConfigAppliesDefaults(t *testing.T) {
	t.Parallel()

	sqlite := validSQLite()
	if got := sqlite.timeout(); got != 10*time.Second {
		t.Errorf("timeout() = %v, want 10s", got)
	}

	mysql := validMySQL()
	if got := mysql.MySQL.port(); got != 3306 {
		t.Errorf("MySQL port() = %d, want 3306", got)
	}

	postgres := validPostgres()
	if got := postgres.Postgres.port(); got != 5432 {
		t.Errorf("PostgreSQL port() = %d, want 5432", got)
	}
}

func TestConfigKeepsExplicitValues(t *testing.T) {
	t.Parallel()

	mysql := validMySQL()
	mysql.MySQL.Port = 3307
	mysql.Timeout = 2 * time.Second

	if got := mysql.MySQL.port(); got != 3307 {
		t.Errorf("MySQL port() = %d, want 3307", got)
	}
	if got := mysql.timeout(); got != 2*time.Second {
		t.Errorf("timeout() = %v, want 2s", got)
	}
}

func TestConfigMigrationsAreOptional(t *testing.T) {
	t.Parallel()

	cfg := validSQLite()
	if cfg.Migrations != nil {
		t.Fatalf("Migrations = %v, want nil", cfg.Migrations)
	}
	if err := cfg.validate(); err != nil {
		t.Errorf("validate() with nil Migrations = %v, want nil", err)
	}
}

// ExampleConfig shows how to fill Config for the three supported databases.
// Port 0 applies the default (3306 for MySQL, 5432 for PostgreSQL) and
// Timeout 0 applies the default of 10 seconds. The credentials are
// illustrative; no external configuration file is required.
func ExampleConfig() {
	_ = Config{
		Type:   DBTypeSQLite,
		Driver: "sqlite",
		SQLite: &SQLiteConfig{DBPath: "dev.db"},
	}

	_ = Config{
		Type:   DBTypeMySQL,
		Driver: "mysql",
		MySQL: &MySQLConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
		},
		Timeout: 5 * time.Second,
	}

	_ = Config{
		Type:   DBTypePostgres,
		Driver: "pgx",
		Postgres: &PostgresConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
		},
		Timeout: 5 * time.Second,
		// Migrations: nil (not configured).
	}

	// Output:
}
