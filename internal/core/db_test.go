package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOpenReleasesConnectionOnPingFailure(t *testing.T) {
	cause := &readFailure{}
	d := &testDriver{ping: func(context.Context) error { return cause }}
	db, err := Open(context.Background(), driverConfig(d))
	if db != nil {
		db.Close()
		t.Fatal("unexpected DB")
	}
	if !errors.Is(err, ErrConnectionFailed) || !errors.Is(err, cause) {
		t.Fatalf("Open = %v", err)
	}
	if d.opens.Load() != 1 || d.closes.Load() != 1 {
		t.Fatal("opened resource was not released")
	}
}

func TestOpenCancellationDuringPing(t *testing.T) {
	entered := make(chan struct{})
	d := &testDriver{ping: func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }}
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
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("Open = %v", err)
	}
	if d.opens.Load() != 1 || d.closes.Load() != 1 {
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
		d := &testDriver{ping: func(ctx context.Context) error {
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
		if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrConnectionFailed) {
			t.Fatalf("Open = %v", err)
		}
		if (shortContext && !actualDeadline.Equal(parentDeadline)) || (!shortContext && !actualDeadline.Before(parentDeadline)) {
			t.Fatal("shorter deadline was ignored")
		}
		if d.closes.Load() != 1 {
			t.Fatal("deadline leaked connection")
		}
	}
}

func TestOpenBudgetIncludesDriverPreparation(t *testing.T) {
	// OpenConnector has no context argument. Waiting for a timer here models
	// non-cooperative preparation, not synchronization of a concurrent test.
	d := &testDriver{prepare: func() { <-time.After(30 * time.Millisecond) }}
	cfg := driverConfig(d)
	cfg.Timeout = 10 * time.Millisecond
	db, err := Open(context.Background(), cfg)
	if db != nil {
		db.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Open = %v, want expired preparation budget", err)
	}
	if d.opens.Load() != 0 {
		t.Fatal("connection attempted after budget expired")
	}
}
