package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
)

// Each test registers its own state. Hooks are configured before opening;
// counters and channels make concurrent scenarios observable without sleeps.
type testDriver struct {
	prepare                           func()
	ping                              func(context.Context) error
	exec                              func(context.Context) error
	query                             func() (driver.Rows, error)
	queryContext                      func(context.Context) (driver.Rows, error)
	commitErr, rollbackErr            error
	rolledBack                        chan struct{}
	opens, closes, commits, rollbacks atomic.Int32
}

var driverSequence atomic.Int64

func driverConfig(d *testDriver) Config {
	name := fmt.Sprintf("krios_test_%d", driverSequence.Add(1))
	sql.Register(name, d)
	return Config{Type: DBTypeSQLite, Driver: name, SQLite: &SQLiteConfig{DBPath: "unused"}}
}

func openDriver(t *testing.T, d *testDriver) *DB {
	t.Helper()
	db, err := Open(context.Background(), driverConfig(d))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func beginDriver(t *testing.T, db *DB, ctx context.Context) *Tx {
	t.Helper()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

func (d *testDriver) Open(string) (driver.Conn, error) { d.opens.Add(1); return &testConn{d}, nil }
func (d *testDriver) OpenConnector(string) (driver.Connector, error) {
	if d.prepare != nil {
		d.prepare()
	}
	return testConnector{d}, nil
}

type testConnector struct{ d *testDriver }

func (c testConnector) Driver() driver.Driver { return c.d }
func (c testConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.d.Open("")
}

type testConn struct{ d *testDriver }

func (c *testConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected Prepare") }
func (c *testConn) Close() error                        { c.d.closes.Add(1); return nil }
func (c *testConn) Begin() (driver.Tx, error)           { return testTx{c.d}, nil }
func (c *testConn) Ping(ctx context.Context) error {
	if c.d.ping != nil {
		return c.d.ping(ctx)
	}
	return nil
}
func (c *testConn) ExecContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.d.exec != nil {
		if err := c.d.exec(ctx); err != nil {
			return nil, err
		}
	}
	return driver.RowsAffected(1), nil
}
func (c *testConn) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.d.queryContext != nil {
		return c.d.queryContext(ctx)
	}
	if c.d.query != nil {
		return c.d.query()
	}
	return &testRows{}, nil
}

type testTx struct{ d *testDriver }

func (tx testTx) Commit() error { tx.d.commits.Add(1); return tx.d.commitErr }
func (tx testTx) Rollback() error {
	tx.d.rollbacks.Add(1)
	if tx.d.rolledBack != nil {
		close(tx.d.rolledBack)
	}
	return tx.d.rollbackErr
}

type testRows struct {
	values            []driver.Value
	nextErr, closeErr error
	index             int
	closes            atomic.Int32
}

func (*testRows) Columns() []string { return []string{"value"} }
func (r *testRows) Close() error    { r.closes.Add(1); return r.closeErr }
func (r *testRows) Next(dest []driver.Value) error {
	if r.index < len(r.values) {
		dest[0] = r.values[r.index]
		r.index++
		return nil
	}
	if r.nextErr != nil {
		return r.nextErr
	}
	return io.EOF
}
