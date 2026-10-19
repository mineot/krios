package core

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"
)

func assertTxDone(t *testing.T, tx *Tx) {
	t.Helper()
	ctx := context.Background()
	_, execErr := tx.ExecContext(ctx, "write")
	_, queryErr := tx.QueryContext(ctx, "read")
	var value int
	for _, err := range []error{execErr, queryErr, tx.QueryRowContext(ctx, "read").Scan(&value), tx.Commit(ctx), tx.Rollback(ctx)} {
		if !errors.Is(err, ErrTxDone) {
			t.Errorf("operation after end = %v, want ErrTxDone", err)
		}
	}
}

func TestTxFinalizationFailureIsTerminal(t *testing.T) {
	for _, commit := range []bool{true, false} {
		name := "rollback"
		if commit {
			name = "commit"
		}
		t.Run(name, func(t *testing.T) {
			cause := &readFailure{}
			d := &testDriver{commitErr: cause, rollbackErr: cause}
			tx := beginDriver(t, openDriver(t, d), context.Background())
			finish := tx.Rollback
			if commit {
				finish = tx.Commit
			}
			if err := finish(context.Background()); !errors.Is(err, cause) {
				t.Fatalf("finish = %v", err)
			}
			assertTxDone(t, tx)
			if d.commits.Load()+d.rollbacks.Load() != 1 {
				t.Fatal("native finalization repeated")
			}
		})
	}
}

func TestRollbackIgnoresCanceledContext(t *testing.T) {
	for _, expired := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if expired {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}
		cancel()
		d := &testDriver{}
		tx := beginDriver(t, openDriver(t, d), context.Background())
		if err := tx.Rollback(ctx); err != nil {
			t.Errorf("Rollback = %v", err)
		}
		if d.rollbacks.Load() != 1 {
			t.Error("cleanup never reached driver")
		}
		assertTxDone(t, tx)
	}
}

func TestBlockedCommitPreservesBothErrors(t *testing.T) {
	for _, query := range []bool{false, true} {
		t.Run(map[bool]string{false: "exec", true: "query"}[query], func(t *testing.T) {
			first, later := errors.New("first statement"), errors.New("later statement")
			rollbackErr := &readFailure{}
			calls := 0
			fail := func() error {
				calls++
				if calls == 1 {
					return first
				}
				return later
			}
			d := &testDriver{rollbackErr: rollbackErr, exec: func(context.Context) error { return fail() }, query: func() (driver.Rows, error) { return nil, fail() }}
			tx := beginDriver(t, openDriver(t, d), context.Background())
			for range 2 {
				if query {
					_, _ = tx.QueryContext(context.Background(), "read")
				} else {
					_, _ = tx.ExecContext(context.Background(), "write")
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := tx.Commit(ctx)
			if !errors.Is(err, first) || !errors.Is(err, rollbackErr) || errors.Is(err, later) {
				t.Errorf("Commit = %v; lost first or cleanup error", err)
			}
			var typed *readFailure
			if !errors.As(err, &typed) {
				t.Error("lost rollback error type")
			}
			if d.commits.Load() != 0 || d.rollbacks.Load() != 1 {
				t.Error("wrong native finalization")
			}
			assertTxDone(t, tx)
		})
	}
}

func TestCanceledCommitCanBeRetriedBeforeNativeCall(t *testing.T) {
	d := &testDriver{}
	tx := beginDriver(t, openDriver(t, d), context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := tx.Commit(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if d.commits.Load()+d.rollbacks.Load() != 0 {
		t.Fatal("unexpected native finalization")
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.commits.Load() != 1 {
		t.Fatal("commit did not reach driver")
	}
}

func TestBeginCancellationNormalizesNativeTerminalState(t *testing.T) {
	d := &testDriver{rolledBack: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tx := beginDriver(t, openDriver(t, d), ctx)
	cancel()
	select {
	case <-d.rolledBack:
	case <-time.After(5 * time.Second):
		t.Fatal("native rollback did not finish")
	}
	// QueryRow defers the native ErrTxDone until Scan.
	var value int
	if err := tx.QueryRowContext(context.Background(), "read").Scan(&value); !errors.Is(err, ErrTxDone) {
		t.Errorf("Scan = %v", err)
	}
	assertTxDone(t, tx)
	if d.rollbacks.Load() != 1 {
		t.Fatal("rollback repeated")
	}
}

func TestLateReadFailureRequiresConsumerRollback(t *testing.T) {
	for _, row := range []bool{false, true} {
		d := &testDriver{query: func() (driver.Rows, error) { return &testRows{nextErr: &readFailure{}}, nil }}
		tx := beginDriver(t, openDriver(t, d), context.Background())
		if row {
			var value int
			if tx.QueryRowContext(context.Background(), "read").Scan(&value) == nil {
				t.Fatal("missing read error")
			}
		} else {
			rows, err := tx.QueryContext(context.Background(), "read")
			if err != nil {
				t.Fatal(err)
			}
			if rows.Next() || rows.Err() == nil {
				t.Fatal("missing iteration error")
			}
			_ = rows.Close()
		}
		// Demonstrate the documented boundary: ignoring a late error does not
		// poison Commit. Consumers must explicitly roll back instead.
		if err := tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		if d.commits.Load() != 1 {
			t.Fatal("late error unexpectedly blocked commit")
		}
	}
}

func TestFinalizationClosesOpenResults(t *testing.T) {
	for _, commit := range []bool{false, true} {
		native := &testRows{values: []driver.Value{int64(1), int64(2)}}
		d := &testDriver{query: func() (driver.Rows, error) { return native, nil }}
		tx := beginDriver(t, openDriver(t, d), context.Background())
		rows, err := tx.QueryContext(context.Background(), "read")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if !rows.Next() {
			t.Fatal("missing row")
		}
		finish := tx.Rollback
		if commit {
			finish = tx.Commit
		}
		if err := finish(context.Background()); err != nil {
			t.Fatal(err)
		}
		if native.closes.Load() != 1 || rows.Next() {
			t.Fatal("result still open after finalization")
		}
		assertTxDone(t, tx)
	}
}

func TestConcurrentFinalizationWaitsForExecution(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	d := &testDriver{exec: func(ctx context.Context) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx := beginDriver(t, openDriver(t, d), ctx)
	execDone := make(chan error, 1)
	go func() { _, err := tx.ExecContext(ctx, "write"); execDone <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	finishDone := make(chan error, 2)
	go func() { finishDone <- tx.Commit(context.Background()) }()
	go func() { finishDone <- tx.Rollback(context.Background()) }()
	close(release)
	if err := <-execDone; err != nil {
		t.Fatal(err)
	}
	first, second := <-finishDone, <-finishDone
	if !((first == nil && errors.Is(second, ErrTxDone)) || (second == nil && errors.Is(first, ErrTxDone))) {
		t.Fatalf("finalization = %v / %v", first, second)
	}
	if d.commits.Load()+d.rollbacks.Load() != 1 {
		t.Fatal("native transaction ended more than once")
	}
}
