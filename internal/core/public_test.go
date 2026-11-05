package core_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/mineot/krios"
)

type connector struct{}

func (connector) Connect(context.Context) (driver.Conn, error) { return conn{}, nil }
func (connector) Driver() driver.Driver                        { return drv{} }

type drv struct{}

func (drv) Open(string) (driver.Conn, error) { return conn{}, nil }

type conn struct{}

func (conn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected SQL") }
func (conn) Begin() (driver.Tx, error)           { return nil, errors.New("unexpected transaction") }
func (conn) Close() error                        { return nil }
func (conn) Ping(context.Context) error          { return nil }

func TestPublicAPI(t *testing.T) {
	db := sql.OpenDB(connector{})
	defer db.Close()
	for _, dialect := range []krios.Dialect{krios.DialectSQLite3, krios.DialectPostgreSQL, krios.DialectMySQL} {
		var client *krios.Client
		var err error
		client, err = krios.New(krios.Config{Db: db, Context: context.Background(), DbVersion: 1, Dialect: dialect, Migrations: []krios.Migration{{Version: 1, Up: "absent.sql", Down: "absent.sql", NonTransactional: true}}})
		if client == nil || err != nil {
			t.Fatalf("New = %v, %v", client, err)
		}
	}
	client, err := krios.New(krios.Config{})
	if client != nil || !errors.Is(err, krios.ErrInvalidConfig) {
		t.Fatalf("invalid New = %v, %v", client, err)
	}
	if krios.ErrInvalidQuery == nil || krios.ErrInvalidMigration == nil || errors.Is(krios.ErrInvalidQuery, krios.ErrInvalidMigration) {
		t.Fatal("invalid public error categories")
	}
	if string(krios.DialectSQLite3) != "sqlite3" || string(krios.DialectPostgreSQL) != "postgres" || string(krios.DialectMySQL) != "mysql" {
		t.Fatal("dialect values changed")
	}
}
