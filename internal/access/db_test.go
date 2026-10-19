package access

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/testutil"
	"github.com/mineot/krios/internal/transaction"
)

func driverConfig(d *testutil.Driver) config.Config { return testutil.Config(d) }

func openDriver(t *testing.T, d *testutil.Driver) *DB {
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

func beginDriver(t *testing.T, db *DB, ctx context.Context) *transaction.Tx {
	t.Helper()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

	return tx
}

func TestOpenReleasesConnectionOnPingFailure(t *testing.T) {
	cause := &testutil.ReadFailure{}
	d := &testutil.Driver{Ping: func(context.Context) error { return cause }}
	db, err := Open(context.Background(), driverConfig(d))
	if db != nil {
		db.Close()
		t.Fatal("unexpected DB")
	}
	if !errors.Is(err, errs.ErrConnectionFailed) || !errors.Is(err, cause) {
		t.Fatalf("Open = %v", err)
	}
	if d.Opens.Load() != 1 || d.Closes.Load() != 1 {
		t.Fatal("opened resource was not released")
	}
}

func TestOpenCancellationDuringPing(t *testing.T) {
	entered := make(chan struct{})
	d := &testutil.Driver{Ping: func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := driverConfig(d)
	done := make(chan error, 1)
	go func() {
		db, err := Open(ctx, cfg)
		if db != nil {
			db.Close()
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("ping not reached")
	}
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) || errors.Is(err, errs.ErrConnectionFailed) {
		t.Fatalf("Open = %v", err)
	}
	if d.Opens.Load() != 1 || d.Closes.Load() != 1 {
		t.Fatal("canceled ping leaked connection")
	}
}

func TestOpenUsesShortestDeadline(t *testing.T) {
	for _, shortContext := range []bool{false, true} {
		ctxDuration, cfgDuration := time.Second, 20*time.Millisecond
		if shortContext {
			ctxDuration, cfgDuration = cfgDuration, ctxDuration
		}
		ctx, cancel := context.WithTimeout(context.Background(), ctxDuration)
		parentDeadline, _ := ctx.Deadline()
		var actualDeadline time.Time
		d := &testutil.Driver{Ping: func(ctx context.Context) error {
			actualDeadline, _ = ctx.Deadline()
			<-ctx.Done()
			return ctx.Err()
		}}
		cfg := driverConfig(d)
		cfg.Timeout = cfgDuration
		db, err := Open(ctx, cfg)
		cancel()
		if db != nil {
			db.Close()
		}
		if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errs.ErrConnectionFailed) {
			t.Fatalf("Open = %v", err)
		}
		if (shortContext && !actualDeadline.Equal(parentDeadline)) || (!shortContext && !actualDeadline.Before(parentDeadline)) {
			t.Fatal("shorter deadline was ignored")
		}
		if d.Closes.Load() != 1 {
			t.Fatal("deadline leaked connection")
		}
	}
}

func TestOpenBudgetIncludesDriverPreparation(t *testing.T) {
	// OpenConnector has no context argument. Waiting for a timer here models
	// non-cooperative preparation, not synchronization of a concurrent test.
	d := &testutil.Driver{Prepare: func() { <-time.After(30 * time.Millisecond) }}
	cfg := driverConfig(d)
	cfg.Timeout = 10 * time.Millisecond
	db, err := Open(context.Background(), cfg)
	if db != nil {
		db.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Open = %v, want expired preparation budget", err)
	}
	if d.Opens.Load() != 0 {
		t.Fatal("connection attempted after budget expired")
	}
}

func TestDBEnsureOpenReportsClosed(t *testing.T) {
	t.Parallel()

	open := &DB{}
	if err := open.ensureOpen(); err != nil {
		t.Errorf("ensureOpen() before Close = %v, want nil", err)
	}

	closed := &DB{closed: true}
	if err := closed.ensureOpen(); !errors.Is(err, errs.ErrClosed) {
		t.Errorf("ensureOpen() after Close = %v, want ErrClosed", err)
	}
}
