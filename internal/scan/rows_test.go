package scan_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mineot/krios/internal/access"
	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/testutil"
	"github.com/mineot/krios/internal/transaction"
)

func driverConfig(d *testutil.Driver) config.Config { return testutil.Config(d) }

func openDriver(t *testing.T, d *testutil.Driver) *access.DB {
	t.Helper()

	db, err := access.Open(context.Background(), driverConfig(d))
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

func beginDriver(t *testing.T, db *access.DB, ctx context.Context) *transaction.Tx {
	t.Helper()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

	return tx
}

func assertTxDone(t *testing.T, tx *transaction.Tx) {
	t.Helper()
	ctx := context.Background()
	_, execErr := tx.ExecContext(ctx, "write")
	_, queryErr := tx.QueryContext(ctx, "read")
	var value int
	for _, err := range []error{execErr, queryErr, tx.QueryRowContext(ctx, "read").Scan(&value), tx.Commit(ctx), tx.Rollback(ctx)} {
		if !errors.Is(err, errs.ErrTxDone) {
			t.Errorf("operation after end = %v, want errs.ErrTxDone", err)
		}
	}
}

func TestQueryRowErrors(t *testing.T) {
	for _, transaction := range []bool{false, true} {
		for _, scenario := range []string{"canceled", "deadline", "closed", "empty", "driver", "iteration", "conversion"} {
			t.Run(fmt.Sprintf("tx=%v/%s", transaction, scenario), func(t *testing.T) {
				cause := &testutil.ReadFailure{}
				d := &testutil.Driver{}
				if scenario == "driver" {
					d.Query = func() (driver.Rows, error) { return nil, cause }
				}
				if scenario == "iteration" {
					d.Query = func() (driver.Rows, error) { return &testutil.Rows{NextErr: cause}, nil }
				}
				if scenario == "conversion" {
					d.Query = func() (driver.Rows, error) { return &testutil.Rows{Values: []driver.Value{"not an integer"}}, nil }
				}
				db := openDriver(t, d)
				query := db.QueryRowContext
				finish := func() { _ = db.Close() }
				want := error(sql.ErrNoRows)
				if transaction {
					tx := beginDriver(t, db, context.Background())
					query = tx.QueryRowContext
					finish = func() { _ = tx.Rollback(context.Background()) }
				}
				ctx := context.Background()
				switch scenario {
				case "canceled":
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
					want = context.Canceled
				case "deadline":
					var cancel context.CancelFunc
					ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
					defer cancel()
					want = context.DeadlineExceeded
				case "closed":
					finish()
					want = errs.ErrClosed
					if transaction {
						want = errs.ErrTxDone
					}
				case "driver", "iteration":
					want = cause
				}
				var value int
				err := query(ctx, "SELECT value").Scan(&value)
				if scenario == "conversion" {
					if err == nil || errors.Is(err, sql.ErrNoRows) {
						t.Fatalf("conversion error = %v", err)
					}
					return
				}
				if !errors.Is(err, want) {
					t.Fatalf("Scan = %v, want %v", err, want)
				}
				if scenario == "driver" || scenario == "iteration" {
					var typed *testutil.ReadFailure
					if !errors.As(err, &typed) {
						t.Fatal("lost error type")
					}
				}
			})
		}
	}
}

func TestRowsLifecycle(t *testing.T) {
	for _, scenario := range []string{"partial", "eof", "iteration", "close", "conversion"} {
		t.Run(scenario, func(t *testing.T) {
			cause := errors.New("driver failure")
			native := &testutil.Rows{Values: []driver.Value{int64(7)}}
			if scenario == "iteration" {
				native.NextErr = cause
			}
			if scenario == "close" {
				native.CloseErr = cause
			}
			if scenario == "conversion" {
				native.Values[0] = "invalid integer"
			}
			db := openDriver(t, &testutil.Driver{Query: func() (driver.Rows, error) { return native, nil }})
			rows, err := db.QueryContext(context.Background(), "SELECT value")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if !rows.Next() {
				t.Fatal("missing first row")
			}
			var value int
			err = rows.Scan(&value)
			if (err != nil) != (scenario == "conversion") {
				t.Fatalf("Scan = %v", err)
			}
			if scenario == "iteration" || scenario == "eof" {
				if rows.Next() {
					t.Fatal("unexpected second row")
				}
				if native.Closes.Load() != 1 {
					t.Fatal("iteration did not close resource")
				}
			}
			closeErr := rows.Close()
			if (closeErr != nil) != (scenario == "close") {
				t.Fatalf("Close = %v", closeErr)
			}
			want := error(nil)
			if scenario == "iteration" || scenario == "close" {
				want = cause
			}
			if !errors.Is(rows.Err(), want) {
				t.Fatalf("Err after Close = %v, want %v", rows.Err(), want)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if native.Closes.Load() != 1 {
				t.Fatal("resource closed more than once")
			}
		})
	}
}
