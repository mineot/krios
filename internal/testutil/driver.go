// Package testutil provides the deterministic database/sql driver used by the
// internal test suites. It depends only on the standard library and config, so
// each layer can build its own fixtures without an import cycle.
package testutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/mineot/krios/internal/config"
)

// Driver is a database/sql driver with configurable hooks and observable
// counters. Configure the hooks before opening; counters and channels make
// concurrent scenarios observable without sleeps. Each test registers its own
// state through Config.
type Driver struct {
	Prepare      func()
	Ping         func(context.Context) error
	Exec         func(context.Context) error
	Query        func() (driver.Rows, error)
	QueryContext func(context.Context) (driver.Rows, error)
	CommitErr    error
	RollbackErr  error
	RolledBack   chan struct{}

	Opens     atomic.Int32
	Closes    atomic.Int32
	Commits   atomic.Int32
	Rollbacks atomic.Int32

	lastQuery atomic.Value // string
	lastArgs  atomic.Value // []driver.NamedValue
}

// LastQuery returns the query text of the most recent ExecContext or
// QueryContext call, or "" when none happened.
func (d *Driver) LastQuery() string {
	if v := d.lastQuery.Load(); v != nil {
		return v.(string)
	}

	return ""
}

// LastArgs returns the bound arguments of the most recent ExecContext or
// QueryContext call in driver order, or nil when none happened.
func (d *Driver) LastArgs() []driver.NamedValue {
	if v := d.lastArgs.Load(); v != nil {
		return v.([]driver.NamedValue)
	}

	return nil
}

var driverSequence atomic.Int64

// Config registers d under a unique driver name and returns a valid SQLite
// configuration that uses it.
func Config(d *Driver) config.Config {
	name := fmt.Sprintf("krios_test_%d", driverSequence.Add(1))
	sql.Register(name, d)
	return config.Config{Type: config.DBTypeSQLite, Driver: name, SQLite: &config.SQLiteConfig{DBPath: "unused"}}
}

func (d *Driver) Open(string) (driver.Conn, error) {
	d.Opens.Add(1)

	return &conn{d}, nil
}

func (d *Driver) OpenConnector(string) (driver.Connector, error) {
	if d.Prepare != nil {
		d.Prepare()
	}

	return connector{d}, nil
}

type connector struct{ d *Driver }

func (c connector) Driver() driver.Driver { return c.d }

func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return c.d.Open("")
}

type conn struct{ d *Driver }

func (c *conn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected Prepare") }
func (c *conn) Close() error                        { c.d.Closes.Add(1); return nil }
func (c *conn) Begin() (driver.Tx, error)           { return connTx{c.d}, nil }

func (c *conn) Ping(ctx context.Context) error {
	if c.d.Ping != nil {
		return c.d.Ping(ctx)
	}

	return nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.d.lastQuery.Store(query)
	c.d.lastArgs.Store(args)

	if c.d.Exec != nil {
		if err := c.d.Exec(ctx); err != nil {
			return nil, err
		}
	}

	return driver.RowsAffected(1), nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.lastQuery.Store(query)
	c.d.lastArgs.Store(args)

	if c.d.QueryContext != nil {
		return c.d.QueryContext(ctx)
	}

	if c.d.Query != nil {
		return c.d.Query()
	}

	return &Rows{}, nil
}

type connTx struct{ d *Driver }

func (tx connTx) Commit() error {
	tx.d.Commits.Add(1)

	return tx.d.CommitErr
}

func (tx connTx) Rollback() error {
	tx.d.Rollbacks.Add(1)

	if tx.d.RolledBack != nil {
		close(tx.d.RolledBack)
	}

	return tx.d.RollbackErr
}

// Rows is a single-column driver.Rows fixture. Next serves Values before
// returning NextErr and then io.EOF; Close returns CloseErr.
type Rows struct {
	Values   []driver.Value
	NextErr  error
	CloseErr error

	Closes atomic.Int32

	index int
}

func (*Rows) Columns() []string { return []string{"value"} }

func (r *Rows) Close() error {
	r.Closes.Add(1)

	return r.CloseErr
}

func (r *Rows) Next(dest []driver.Value) error {
	if r.index < len(r.Values) {
		dest[0] = r.Values[r.index]
		r.index++

		return nil
	}

	if r.NextErr != nil {
		return r.NextErr
	}

	return io.EOF
}

// ReadFailure is an error used to exercise late read failures and error
// preservation through the typed-error helpers.
type ReadFailure struct{}

func (*ReadFailure) Error() string { return "read failed" }
