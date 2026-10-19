package tests

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

type failingDriver struct{}

func (failingDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("connection refused")
}

var _ driver.Driver = failingDriver{}

func init() {
	sql.Register("krios_fail", failingDriver{})
}

func sqliteConfig(path string) krios.Config {
	return krios.Config{
		Type:   krios.DBTypeSQLite,
		Driver: "sqlite",
		SQLite: &krios.SQLiteConfig{DBPath: path},
	}
}

func TestOpenAndCloseSQLite(t *testing.T) {
	t.Parallel()

	cfg := sqliteConfig(filepath.Join(t.TempDir(), "krios.db"))
	db, err := krios.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() = %v, want nil", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("first Close() = %v, want nil", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("second Close() = %v, want nil (idempotent)", err)
	}
}

func TestOpenInvalidConfig(t *testing.T) {
	t.Parallel()

	if _, err := krios.Open(context.Background(), krios.Config{}); !errors.Is(err, krios.ErrInvalidConfig) {
		t.Fatalf("Open() = %v, want ErrInvalidConfig", err)
	}
}

func TestOpenDriverNotLoaded(t *testing.T) {
	t.Parallel()

	cfg := sqliteConfig(filepath.Join(t.TempDir(), "krios.db"))
	cfg.Driver = "krios_missing"
	if _, err := krios.Open(context.Background(), cfg); !errors.Is(err, krios.ErrDriverNotLoaded) {
		t.Fatalf("Open() = %v, want ErrDriverNotLoaded", err)
	}
}

func TestOpenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := sqliteConfig(filepath.Join(t.TempDir(), "krios.db"))
	_, err := krios.Open(ctx, cfg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Open() = %v, want context.Canceled", err)
	}
	if errors.Is(err, krios.ErrConnectionFailed) {
		t.Errorf("Open() = %v, must not wrap ErrConnectionFailed on cancellation", err)
	}
}

func TestOpenDeadlineExceeded(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	cfg := sqliteConfig(filepath.Join(t.TempDir(), "krios.db"))
	_, err := krios.Open(ctx, cfg)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Open() = %v, want context.DeadlineExceeded", err)
	}
	if errors.Is(err, krios.ErrConnectionFailed) {
		t.Errorf("Open() = %v, must not wrap ErrConnectionFailed on deadline", err)
	}
}

func TestOpenConnectionFailure(t *testing.T) {
	t.Parallel()

	cfg := krios.Config{
		Type:   krios.DBTypeMySQL,
		Driver: "krios_fail",
		MySQL: &krios.MySQLConfig{
			Host:     "127.0.0.1",
			User:     "app",
			Password: "secret",
			DBName:   "app_db",
		},
	}
	if _, err := krios.Open(context.Background(), cfg); !errors.Is(err, krios.ErrConnectionFailed) {
		t.Fatalf("Open() = %v, want ErrConnectionFailed", err)
	}
}
