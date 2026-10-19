package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"
)

type readFailure struct{}

func (*readFailure) Error() string { return "read failed" }

func TestQueryRowErrors(t *testing.T) {
	for _, transaction := range []bool{false, true} {
		for _, scenario := range []string{"canceled", "deadline", "closed", "empty", "driver", "iteration", "conversion"} {
			t.Run(fmt.Sprintf("tx=%v/%s", transaction, scenario), func(t *testing.T) {
				cause := &readFailure{}
				d := &testDriver{}
				if scenario == "driver" {
					d.query = func() (driver.Rows, error) { return nil, cause }
				}
				if scenario == "iteration" {
					d.query = func() (driver.Rows, error) { return &testRows{nextErr: cause}, nil }
				}
				if scenario == "conversion" {
					d.query = func() (driver.Rows, error) { return &testRows{values: []driver.Value{"not an integer"}}, nil }
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
					want = ErrClosed
					if transaction {
						want = ErrTxDone
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
					var typed *readFailure
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
			native := &testRows{values: []driver.Value{int64(7)}}
			if scenario == "iteration" {
				native.nextErr = cause
			}
			if scenario == "close" {
				native.closeErr = cause
			}
			if scenario == "conversion" {
				native.values[0] = "invalid integer"
			}
			db := openDriver(t, &testDriver{query: func() (driver.Rows, error) { return native, nil }})
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
				if native.closes.Load() != 1 {
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
			if native.closes.Load() != 1 {
				t.Fatal("resource closed more than once")
			}
		})
	}
}

type testResult struct {
	value int64
	err   error
}

func (r testResult) LastInsertId() (int64, error) { return r.value, r.err }
func (r testResult) RowsAffected() (int64, error) { return r.value, r.err }
func TestResultAvailability(t *testing.T) {
	for _, tc := range []struct {
		name      string
		result    sql.Result
		value     int64
		available bool
	}{
		{"zero", testResult{}, 0, true}, {"value", testResult{value: 7}, 7, true},
		{"unsupported", testResult{err: errors.New("unsupported")}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := wrapResult(tc.result)
			for _, method := range []func() (int64, bool){r.LastInsertId, r.RowsAffected} {
				if n, ok := method(); n != tc.value || ok != tc.available {
					t.Fatalf("result = %d, %v", n, ok)
				}
			}
		})
	}
}
