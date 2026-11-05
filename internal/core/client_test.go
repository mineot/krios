package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type probe struct {
	pings  atomic.Int32
	closes atomic.Int32
	opens  atomic.Int32
	ping   func(context.Context) error
}

type testConnector struct{ p *probe }

func (c testConnector) Connect(context.Context) (driver.Conn, error) {
	c.p.opens.Add(1)
	return testConn{c.p}, nil
}

func (c testConnector) Driver() driver.Driver { return testDriver{c.p} }

type testDriver struct{ p *probe }

func (d testDriver) Open(string) (driver.Conn, error) {
	return testConnector{d.p}.Connect(context.Background())
}

type testConn struct{ p *probe }

func (c testConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected SQL") }
func (c testConn) Begin() (driver.Tx, error)           { return nil, errors.New("unexpected transaction") }
func (c testConn) Close() error                        { c.p.closes.Add(1); return nil }

func (c testConn) Ping(ctx context.Context) error {
	c.p.pings.Add(1)
	if c.p.ping != nil {
		return c.p.ping(ctx)
	}
	return nil
}

func setup(t *testing.T) (Config, *probe) {
	t.Helper()
	p := &probe{}
	db := sql.OpenDB(testConnector{p})
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return Config{Db: db, Context: context.Background(), Dialect: DialectPostgreSQL, DbVersion: 1}, p
}

func TestInvalidConfigBeforePing(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Config)
	}{
		{"nil database", func(c *Config) { c.Db = nil }},
		{"nil context", func(c *Config) { c.Context = nil }},
		{"empty dialect", func(c *Config) { c.Dialect = "" }},
		{"unknown dialect", func(c *Config) { c.Dialect = "password=secret" }},
		{"negative timeout", func(c *Config) { c.Timeout = -1 }},
		{"zero version", func(c *Config) { c.DbVersion = 0 }},
		{"overflow version", func(c *Config) { c.DbVersion = uint64(math.MaxInt64) + 1 }},
		{"max uint version", func(c *Config) { c.DbVersion = math.MaxUint64 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, p := setup(t)
			tc.change(&cfg)
			cfg.Migrations = []Migration{{Up: "private-script-secret.sql"}}
			client, err := New(cfg)
			if client != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("New = %v, %v", client, err)
			}
			if p.pings.Load() != 0 || p.opens.Load() != 0 {
				t.Fatal("invalid config accessed database")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("configuration leaked: %v", err)
			}
		})
	}
}

func TestValidConfigurations(t *testing.T) {
	for _, dialect := range []Dialect{DialectSQLite3, DialectPostgreSQL, DialectMySQL} {
		for _, version := range []uint64{1, math.MaxInt64} {
			t.Run(string(dialect)+"/"+time.Duration(version).String(), func(t *testing.T) {
				cfg, p := setup(t)
				cfg.Dialect = dialect
				cfg.DbVersion = version
				client, err := New(cfg)
				if err != nil || client == nil {
					t.Fatalf("New = %v, %v", client, err)
				}
				if p.pings.Load() != 1 {
					t.Fatal("expected one ping")
				}
			})
		}
	}
}

type pingError struct{ message string }

func (e *pingError) Error() string { return e.message }

func TestPingFailureAndPoolOwnership(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			cfg, p := setup(t)
			cfg.Timeout = time.Hour
			cause := &pingError{"driver failure"}

			var observed context.Context

			p.ping = func(ctx context.Context) error {
				observed = ctx
				if fail {
					return cause
				}
				return nil
			}

			client, err := New(cfg)

			if fail {
				var typed *pingError
				if client != nil || !errors.Is(err, cause) || !errors.As(err, &typed) || typed != cause {
					t.Fatalf("lost cause: %v", err)
				}
				if errors.Is(err, ErrInvalidConfig) || errors.Is(err, ErrInvalidQuery) || errors.Is(err, ErrInvalidMigration) {
					t.Fatal("driver error misclassified")
				}
				if err.Error() != "krios: ping: driver failure" {
					t.Fatalf("unexpected error context: %v", err)
				}
			} else if err != nil || client == nil {
				t.Fatalf("New = %v, %v", client, err)
			}

			if observed.Err() != context.Canceled {
				t.Fatal("ping context not released")
			}

			if cfg.Context.Err() != nil {
				t.Fatal("parent canceled")
			}

			if p.closes.Load() != 0 {
				t.Fatal("connection closed by creation")
			}

			p.ping = nil

			if err := cfg.Db.PingContext(context.Background()); err != nil {
				t.Fatalf("pool unusable: %v", err)
			}
		})
	}
}

func TestConfigSnapshot(t *testing.T) {
	cfg, _ := setup(t)
	cfg.Migrations = []Migration{{Version: 0, Up: "missing/up.sql", Down: "missing/down.sql", NonTransactional: true}, {Version: 0}}
	original := cfg
	client, err := New(cfg)

	if err != nil {
		t.Fatal(err)
	}

	cfg.Db = nil
	cfg.Context = nil
	cfg.Dialect = DialectMySQL
	cfg.DbVersion = 8
	cfg.Timeout = time.Hour
	cfg.Migrations[0] = Migration{Version: 5}
	cfg.Migrations = nil
	got := client.config

	if got.Db != original.Db || got.Context != original.Context || got.Dialect != original.Dialect || got.DbVersion != original.DbVersion || got.Timeout != original.Timeout {
		t.Fatal("configuration not copied")
	}

	if len(got.Migrations) != 2 || got.Migrations[0] != (Migration{Version: 0, Up: "missing/up.sql", Down: "missing/down.sql", NonTransactional: true}) {
		t.Fatal("migration slice not copied")
	}
	// Invalid migration entries and nonexistent paths are intentionally deferred.
}

func TestCanceledContextSkipsPing(t *testing.T) {
	cfg, p := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg.Context = ctx
	client, err := New(cfg)

	if client != nil || !errors.Is(err, context.Canceled) || errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("New = %v, %v", client, err)
	}

	if p.opens.Load() != 0 || p.pings.Load() != 0 {
		t.Fatal("accessed database after cancellation")
	}
}

func TestPingCancellation(t *testing.T) {
	for _, mode := range []string{"timeout", "parent deadline", "parent cancel"} {
		t.Run(mode, func(t *testing.T) {
			cfg, p := setup(t)
			started := make(chan struct{})
			p.ping = func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }
			var cancel context.CancelFunc
			switch mode {
			case "timeout":
				cfg.Timeout = 20 * time.Millisecond
			case "parent deadline":
				cfg.Context, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()
				cfg.Timeout = time.Hour
			case "parent cancel":
				cfg.Context, cancel = context.WithCancel(context.Background())
				defer cancel()
			}
			done := make(chan error, 1)
			go func() {
				client, err := New(cfg)
				if client != nil {
					done <- errors.New("unexpected client")
					return
				}
				done <- err
			}()
			if mode == "parent cancel" {
				select {
				case <-started:
					cancel()
				case <-time.After(5 * time.Second):
					t.Fatal("ping did not start")
				}
			}
			want := error(context.DeadlineExceeded)
			if mode == "parent cancel" {
				want = context.Canceled
			}
			select {
			case err := <-done:
				if !errors.Is(err, want) || errors.Is(err, ErrInvalidConfig) {
					t.Fatalf("error = %v, want %v", err, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("ping ignored cancellation")
			}
		})
	}
}

func TestOperationContext(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	c := &Client{config: Config{Context: parent}}
	ctx, cancel := c.operationContext()
	if ctx != parent {
		t.Fatal("zero timeout must preserve parent")
	}
	cancel()
	if parent.Err() != nil {
		t.Fatal("zero timeout cleanup canceled parent")
	}
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("zero timeout added deadline")
	}
	c.config.Timeout = time.Hour
	ctx, cancel = c.operationContext()
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("positive timeout has no deadline")
	}
	cancelParent()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("parent cancellation not propagated")
	}
	expired, stop := c.operationContext()
	defer stop()
	if !errors.Is(expired.Err(), context.Canceled) {
		t.Fatal("future operation ignored parent cancellation")
	}
}

func TestEarlierParentDeadline(t *testing.T) {
	deadline := time.Now().Add(time.Hour)
	parent, stop := context.WithDeadline(context.Background(), deadline)
	defer stop()
	c := &Client{config: Config{Context: parent, Timeout: 2 * time.Hour}}
	ctx, cancel := c.operationContext()
	defer cancel()
	got, ok := ctx.Deadline()
	if !ok || !got.Equal(deadline) {
		t.Fatalf("deadline = %v, want %v", got, deadline)
	}
}
