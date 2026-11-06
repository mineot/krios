package clients

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return &fakeConn{}, nil
}

type fakeConnector struct {
	pingErr   error
	blockPing bool
}

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeConn{pingErr: c.pingErr, blockPing: c.blockPing}, nil
}

func (c *fakeConnector) Driver() driver.Driver {
	return fakeDriver{}
}

type fakeConn struct {
	pingErr   error
	blockPing bool
}

func (c *fakeConn) Ping(ctx context.Context) error {
	if c.blockPing {
		<-ctx.Done()
		return ctx.Err()
	}
	return c.pingErr
}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *fakeConn) Close() error {
	return nil
}

func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func newFakeDB(t *testing.T, pingErr error) *sql.DB {
	t.Helper()
	db := sql.OpenDB(&fakeConnector{pingErr: pingErr})
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newBlockingDB(t *testing.T) *sql.DB {
	t.Helper()
	db := sql.OpenDB(&fakeConnector{blockPing: true})
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func validConfig(t *testing.T, db *sql.DB) core.Config {
	t.Helper()
	return core.Config{
		DB:        db,
		DBVersion: 1,
		Dialect:   dialects.SQLite,
		Context:   context.Background(),
	}
}

func TestCheckDialect(t *testing.T) {
	tests := []struct {
		name    string
		dialect dialects.Dialect
		wantErr bool
	}{
		{"sqlite", dialects.SQLite, false},
		{"postgres", dialects.Postgres, false},
		{"mysql", dialects.Mysql, false},
		{"empty", dialects.Dialect(""), true},
		{"unknown", dialects.Dialect("oracle"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDialect(tt.dialect)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkDialect(%q) error = %v, wantErr %v", tt.dialect, err, tt.wantErr)
			}
		})
	}
}

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  func(t *testing.T) core.Config
		wantErr bool
	}{
		{
			name: "nil context",
			config: func(t *testing.T) core.Config {
				cfg := validConfig(t, newFakeDB(t, nil))
				cfg.Context = nil
				return cfg
			},
			wantErr: true,
		},
		{
			name: "nil database",
			config: func(t *testing.T) core.Config {
				cfg := validConfig(t, nil)
				cfg.DB = nil
				return cfg
			},
			wantErr: true,
		},
		{
			name: "invalid dialect",
			config: func(t *testing.T) core.Config {
				cfg := validConfig(t, newFakeDB(t, nil))
				cfg.Dialect = dialects.Dialect("oracle")
				return cfg
			},
			wantErr: true,
		},
		{
			name: "zero db version",
			config: func(t *testing.T) core.Config {
				cfg := validConfig(t, newFakeDB(t, nil))
				cfg.DBVersion = 0
				return cfg
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: func(t *testing.T) core.Config {
				cfg := validConfig(t, newFakeDB(t, nil))
				cfg.Timeout = -time.Second
				return cfg
			},
			wantErr: true,
		},
		{
			name: "cancelled context",
			config: func(t *testing.T) core.Config {
				cfg := validConfig(t, newFakeDB(t, nil))
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				cfg.Context = ctx
				return cfg
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: func(t *testing.T) core.Config {
				return validConfig(t, newFakeDB(t, nil))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config(t))

			if (err != nil) != tt.wantErr {
				t.Fatalf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if client != nil {
					t.Fatalf("NewClient() client = %v, want nil", client)
				}
				return
			}
			if client == nil {
				t.Fatal("NewClient() client = nil, want non-nil")
			}
		})
	}
}

func TestNewClientPingFailure(t *testing.T) {
	cfg := validConfig(t, newFakeDB(t, errors.New("ping failed")))

	client, err := NewClient(cfg)
	if err == nil {
		t.Fatal("NewClient() error = nil, want error")
	}
	if client != nil {
		t.Fatalf("NewClient() client = %v, want nil", client)
	}
}

func TestNewClientWithTimeout(t *testing.T) {
	cfg := validConfig(t, newFakeDB(t, nil))
	cfg.Timeout = time.Second

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	if client == nil {
		t.Fatal("NewClient() client = nil, want non-nil")
	}
}

func TestNewClientTimeoutExpires(t *testing.T) {
	cfg := validConfig(t, newBlockingDB(t))
	cfg.Timeout = 10 * time.Millisecond

	client, err := NewClient(cfg)
	if err == nil {
		t.Fatal("NewClient() error = nil, want timeout error")
	}
	if client != nil {
		t.Fatalf("NewClient() client = %v, want nil", client)
	}
}

func TestNewClientCopiesMigrations(t *testing.T) {
	cfg := validConfig(t, newFakeDB(t, nil))
	cfg.Migrations = []core.Migration{
		{Version: 1, Up: "CREATE TABLE a", Down: "DROP TABLE a"},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	if len(client.config.Migrations) != len(cfg.Migrations) {
		t.Fatalf("client migrations len = %d, want %d", len(client.config.Migrations), len(cfg.Migrations))
	}

	cfg.Migrations[0].Up = "CHANGED"

	if got := client.config.Migrations[0].Up; got != "CREATE TABLE a" {
		t.Fatalf("client migration Up = %q, want %q (client shares backing array)", got, "CREATE TABLE a")
	}
}
