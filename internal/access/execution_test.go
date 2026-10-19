package access

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/testutil"
)

func TestExecutorPreservesPublicGuards(t *testing.T) {
	ctx := context.Background()
	cause := errors.New("statement failed")
	d := &testutil.Driver{Exec: func(context.Context) error { return cause }}
	db := openDriver(t, d)
	tx := beginDriver(t, db, ctx)
	var ex executor = tx
	if _, err := ex.ExecContext(ctx, "write"); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	if d.Commits.Load() != 0 || d.Rollbacks.Load() != 1 {
		t.Fatal("executor bypassed transaction policy")
	}
	if _, err := ex.QueryContext(ctx, "read"); !errors.Is(err, errs.ErrTxDone) {
		t.Fatal(err)
	}
	ex = db
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var value int
	if err := ex.QueryRowContext(ctx, "read").Scan(&value); !errors.Is(err, errs.ErrClosed) {
		t.Fatal(err)
	}
}

func TestExecutionCancellationInFlight(t *testing.T) {
	for _, transaction := range []bool{false, true} {
		for _, operation := range []string{"exec", "query", "row"} {
			t.Run(fmt.Sprintf("tx=%v/%s", transaction, operation), func(t *testing.T) {
				entered := make(chan struct{})
				block := func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }
				d := &testutil.Driver{Exec: block, QueryContext: func(ctx context.Context) (driver.Rows, error) { return nil, block(ctx) }}
				db := openDriver(t, d)
				var ex executor = db
				if transaction {
					ex = beginDriver(t, db, context.Background())
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() {
					var err error
					switch operation {
					case "exec":
						_, err = ex.ExecContext(ctx, "write")
					case "query":
						var rows *scan.Rows
						rows, err = ex.QueryContext(ctx, "read")
						if rows != nil {
							_ = rows.Close()
						}
					case "row":
						var value int
						err = ex.QueryRowContext(ctx, "read").Scan(&value)
					}
					done <- err
				}()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("driver not reached")
				}
				cancel()
				if err := <-done; !errors.Is(err, context.Canceled) {
					t.Fatalf("operation = %v", err)
				}
			})
		}
	}
}
