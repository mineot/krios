package tests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

// exampleDatabase isolates every example from the development database.
func exampleDatabase() (*krios.DB, func()) {
	dir, err := os.MkdirTemp("", "krios-example-")
	if err != nil {
		panic(err)
	}
	db, err := krios.Open(context.Background(), krios.Config{
		Type: krios.DBTypeSQLite, Driver: "sqlite",
		SQLite: &krios.SQLiteConfig{DBPath: filepath.Join(dir, "example.db")},
	})
	if err != nil {
		_ = os.RemoveAll(dir)
		panic(err)
	}
	return db, func() { _ = db.Close(); _ = os.RemoveAll(dir) }
}

func Example_openAndRead() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, "SELECT 1 UNION ALL SELECT 2")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			panic(err)
		}
		fmt.Println(value)
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}
	var missing int
	err = db.QueryRowContext(ctx, "SELECT 1 WHERE 1 = 0").Scan(&missing)
	fmt.Println("ausente:", errors.Is(err, sql.ErrNoRows))
	// Output:
	// 1
	// 2
	// ausente: true
}

func Example_transaction() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE entries (id INTEGER PRIMARY KEY)"); err != nil {
		panic(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		panic(err)
	}
	// Cleanup still runs if an operation fails or this function returns early.
	defer tx.Rollback(ctx)
	if _, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (?)", 1); err != nil {
		panic(err)
	}
	if err := tx.Commit(ctx); err != nil {
		panic(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		panic(err)
	}
	fmt.Println("confirmados:", count)
	// Output:
	// confirmados: 1
}

func Example_blockedCommit() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE entries (id INTEGER PRIMARY KEY)"); err != nil {
		panic(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (?)", 1); err != nil {
		panic(err)
	}
	_, statementErr := tx.ExecContext(ctx, "INSERT INTO entries VALUES (?)", 1)
	if statementErr == nil {
		panic("expected duplicate error")
	}
	// Explicit rollback is preferred on failure. This call demonstrates the
	// additional protection when a consumer attempts Commit after that failure.
	fmt.Println("commit bloqueado:", errors.Is(tx.Commit(ctx), statementErr))
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		panic(err)
	}
	fmt.Println("persistidos:", count)
	// Output:
	// commit bloqueado: true
	// persistidos: 0
}

func Example_rollbackAfterReadError() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The transaction lifetime is independent of this operation's context.
	tx, err := db.Begin(context.Background())
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(ctx)
	var number int
	err = tx.QueryRowContext(ctx, "SELECT 'not a number'").Scan(&number)
	if err == nil {
		panic("expected conversion error")
	}
	// Rollback is the recommended response to a read error. Commit would also
	// be blocked: the late error is remembered by the transaction.
	cancel()
	fmt.Println("rollback com contexto cancelado:", tx.Rollback(ctx) == nil)
	// Output:
	// rollback com contexto cancelado: true
}

// exampleEntity maps a table using the entity descriptor: tags name the
// columns and the type name is overridden by TableName.
type exampleEntity struct {
	ID   int64   `krios:"id,pk"`
	Name string  `krios:"name"`
	Bio  *string `krios:"bio"`
}

func (exampleEntity) TableName() string { return "users" }

func Example_structMapping() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (1, 'alice', 'reader'), (2, 'bob', NULL)"); err != nil {
		panic(err)
	}

	// Rows.All maps every remaining row by column name and closes the iterator.
	rows, err := db.QueryContext(ctx, "SELECT id, name, bio FROM users ORDER BY id")
	if err != nil {
		panic(err)
	}
	var users []exampleEntity
	if err := rows.All(&users); err != nil {
		panic(err)
	}
	for _, u := range users {
		bio := "<nil>"
		if u.Bio != nil {
			bio = *u.Bio
		}
		fmt.Println(u.ID, u.Name, bio)
	}

	// A single record maps through Row.Scan; sql.ErrNoRows is preserved.
	var one exampleEntity
	err = db.QueryRowContext(ctx, "SELECT id, name, bio FROM users WHERE id = ?", 1).Scan(&one)
	if err != nil {
		panic(err)
	}
	fmt.Println("registro único:", one.ID, one.Name, *one.Bio)
	err = db.QueryRowContext(ctx, "SELECT id, name, bio FROM users WHERE id = ?", 99).Scan(&one)
	fmt.Println("ausente:", errors.Is(err, sql.ErrNoRows))
	// Output:
	// 1 alice reader
	// 2 bob <nil>
	// registro único: 1 alice reader
	// ausente: true
}
