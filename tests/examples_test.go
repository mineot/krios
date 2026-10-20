package tests

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// exampleNameCount receives grouped query results mapped by output label.
type exampleNameCount struct {
	Name  string `krios:"name"`
	Total int64  `krios:"total"`
}

func Example_aggregationsAndPagination() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (1, 'alice', 'reader'), (2, 'bob', NULL), (3, 'carol', NULL)"); err != nil {
		panic(err)
	}

	users, err := krios.EntityOf[exampleEntity]()
	if err != nil {
		panic(err)
	}

	// Aggregates are expressions with an output alias for mapping by name.
	var grouped []exampleNameCount
	err = db.Select(users).
		Fields(users.Col("name"), krios.Count().As("total")).
		GroupBy(users.Col("name")).
		OrderBy(users.Col("name").Asc()).
		All(ctx, &grouped)
	if err != nil {
		panic(err)
	}
	for _, row := range grouped {
		fmt.Println(row.Name, row.Total)
	}

	// Paginate returns the page and its totals/navigation statistics.
	var page []exampleEntity
	info, err := db.Select(users).
		OrderBy(users.Col("id").Asc()).
		Paginate(ctx, 2, 0, &page)
	if err != nil {
		panic(err)
	}
	fmt.Println(info.Total, info.Page, info.Pages, info.HasNext)
	for _, user := range page {
		fmt.Println(user.ID, user.Name)
	}
	// Output:
	// alice 1
	// bob 1
	// carol 1
	// 3 1 2 true
	// 1 alice
	// 2 bob
}

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

func Example_selectQueries() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (1, 'alice', 'reader'), (2, 'bob', NULL)"); err != nil {
		panic(err)
	}

	users, err := krios.EntityOf[exampleEntity]()
	if err != nil {
		panic(err)
	}

	// Build renders the SQL and bound parameters without executing it.
	query, args, err := db.Select(users).
		Only("id", "name").
		Where(users.Col("name").Like(krios.StartsWith, "a")).
		Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Comparators are typed through the entity; values become parameters.
	var found exampleEntity
	err = db.Select(users).
		Where(krios.And(users.Col("name").Eq("alice"), users.Col("bio").IsNotNull())).
		One(ctx, &found)
	if err != nil {
		panic(err)
	}
	fmt.Println(found.ID, found.Name)
	// Output:
	// SELECT "users"."id", "users"."name" FROM "users" WHERE "users"."name" LIKE ? ESCAPE '!'
	// [a%]
	// 1 alice
}

func Example_insert() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}

	users, err := krios.EntityOf[exampleEntity]()
	if err != nil {
		panic(err)
	}

	res, err := db.Insert(users).
		SetValues(&exampleEntity{ID: 1, Name: "alice", Bio: nil}).
		Exec(ctx)
	if err != nil {
		panic(err)
	}
	n, _ := res.RowsAffected()
	fmt.Println("rows:", n)

	var bio sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT bio FROM users WHERE id = 1").Scan(&bio); err != nil {
		panic(err)
	}
	fmt.Println("bio valid:", bio.Valid)
	// Output:
	// rows: 1
	// bio valid: false
}

func Example_update() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (1, 'alice', 'reader')"); err != nil {
		panic(err)
	}

	users, err := krios.EntityOf[exampleEntity]()
	if err != nil {
		panic(err)
	}

	res, err := db.Update(users).
		SetValues(&exampleEntity{Name: "alice2", Bio: nil}).
		Omit("id").
		Where(users.Col("id").Eq(1)).
		Exec(ctx)
	if err != nil {
		panic(err)
	}
	n, _ := res.RowsAffected()
	fmt.Println("rows:", n)

	var bio sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT bio FROM users WHERE id = 1").Scan(&bio); err != nil {
		panic(err)
	}
	fmt.Println("bio valid:", bio.Valid)

	var name string
	if err := db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = 1").Scan(&name); err != nil {
		panic(err)
	}
	fmt.Println(name)
	// Output:
	// rows: 1
	// bio valid: true
	// alice2
}

func Example_delete() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (1, 'alice', 'reader')"); err != nil {
		panic(err)
	}

	users, err := krios.EntityOf[exampleEntity]()
	if err != nil {
		panic(err)
	}

	res, err := db.Delete(users).Where(users.Col("id").Eq(1)).Exec(ctx)
	if err != nil {
		panic(err)
	}
	n, _ := res.RowsAffected()
	fmt.Println("rows:", n)
	// Output:
	// rows: 1
}

func Example_writeReturning() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}

	users, err := krios.EntityOf[exampleEntity]()
	if err != nil {
		panic(err)
	}

	var ins exampleEntity
	if err := db.Insert(users).SetValues(&exampleEntity{ID: 10, Name: "zoe"}).Returning().One(ctx, &ins); err != nil {
		panic(err)
	}
	fmt.Println(ins.ID, ins.Name)

	var upd []exampleEntity
	if err := db.Update(users).SetValues(&exampleEntity{Name: "zoe2"}).Omit("id", "bio").Where(users.Col("id").Eq(10)).Returning().All(ctx, &upd); err != nil {
		panic(err)
	}
	fmt.Println(len(upd), upd[0].Name)

	var del []exampleEntity
	if err := db.Delete(users).Where(users.Col("id").Eq(10)).Returning().All(ctx, &del); err != nil {
		panic(err)
	}
	fmt.Println(len(del))
	// Output:
	// 10 zoe
	// 1 zoe2
	// 1
}

// exampleStatus validates and formats its own text through the field-level
// interfaces detected by EntityOf.
type exampleStatus string

func (s exampleStatus) Validate() error {
	if s == "" {
		return errors.New("status is required")
	}
	return nil
}

func (s exampleStatus) Format() (string, error) { return strings.ToUpper(string(s)), nil }

type exampleAccount struct {
	ID     int64         `krios:"id,pk"`
	Status exampleStatus `krios:"status"`
	Bio    *string       `krios:"bio"`
}

func (exampleAccount) TableName() string { return "accounts" }

func Example_validationFormatting() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE accounts (id INTEGER PRIMARY KEY, status TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO accounts (id, status, bio) VALUES (1, 'active', 'reader'), (2, '', NULL)"); err != nil {
		panic(err)
	}

	rows, err := db.QueryContext(ctx, "SELECT id, status, bio FROM accounts ORDER BY id")
	if err != nil {
		panic(err)
	}
	var accounts []exampleAccount
	if err := rows.All(&accounts); err != nil {
		panic(err)
	}

	ent, err := krios.EntityOf[exampleAccount]()
	if err != nil {
		panic(err)
	}

	// Validation is explicit and stops at the first invalid record.
	err = ent.ValidateAll(accounts)
	fmt.Println("válidos:", err == nil)
	fmt.Println("erro de validação:", errors.Is(err, krios.ErrValidation))

	// Formatting is presentation-only; the original values are untouched.
	text, err := ent.Format(accounts[0])
	if err != nil {
		panic(err)
	}
	fmt.Println("status:", text["status"], "bio:", text["bio"], "id:", text["id"])
	// Output:
	// válidos: false
	// erro de validação: true
	// status: ACTIVE bio: reader id: 1
}

// exampleRecord demonstrates the E04 completion criterion: reading and
// inspecting values of distinct records through reflection mapping.
type exampleRecord struct {
	ID   int64          `krios:"id,pk"`
	Name string         `krios:"name"`
	Bio  sql.NullString `krios:"bio"`
}

func (exampleRecord) TableName() string { return "records" }

func Example_mappingValidation() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE records (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		panic(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO records (id, name, bio) VALUES (1, 'alice', 'reader'), (2, 'bob', NULL)"); err != nil {
		panic(err)
	}

	rows, err := db.QueryContext(ctx, "SELECT id, name, bio FROM records ORDER BY id")
	if err != nil {
		panic(err)
	}
	// Rows.All maps every row by column name; records stay independent.
	var records []exampleRecord
	if err := rows.All(&records); err != nil {
		panic(err)
	}
	for _, r := range records {
		bio := "<null>"
		if r.Bio.Valid {
			bio = r.Bio.String
		}
		fmt.Println(r.ID, r.Name, bio)
	}

	// A single record maps through Row.Scan; NULL populates sql.NullString.
	var one exampleRecord
	err = db.QueryRowContext(ctx, "SELECT id, name, bio FROM records WHERE id = ?", 2).Scan(&one)
	if err != nil {
		panic(err)
	}
	fmt.Println("nome:", one.Name, "bio válida:", one.Bio.Valid)
	// Output:
	// 1 alice reader
	// 2 bob <null>
	// nome: bob bio válida: false
}

//go:embed testdata/embed_*.sql
var embedSQL embed.FS

type embedUser struct {
	ID   int64  `krios:"id,pk"`
	Name string `krios:"name"`
}

func (embedUser) TableName() string { return "embed_users" }

func Example_sqlFile() {
	db, cleanup := exampleDatabase()
	defer cleanup()
	ctx := context.Background()

	// The consumer embeds the SQL files; the name only selects a resource that
	// is already part of the executable. Loading is eager and fails early.
	schema, err := krios.NewSQLFile(embedSQL, "testdata/embed_schema.sql")
	if err != nil {
		panic(err)
	}
	insert, err := krios.NewSQLFile(embedSQL, "testdata/embed_insert.sql")
	if err != nil {
		panic(err)
	}
	all, err := krios.NewSQLFile(embedSQL, "testdata/embed_all.sql")
	if err != nil {
		panic(err)
	}

	if _, err := db.File(schema).Exec(ctx); err != nil {
		panic(err)
	}

	// Parameters are bound separately, using the markers the driver accepts.
	// SQLite uses '?'.
	for _, name := range []string{"alice", "bob"} {
		if _, err := db.File(insert).Exec(ctx, name); err != nil {
			panic(err)
		}
	}

	// A read resource maps its rows into a slice, by column name.
	var users []embedUser
	if err := db.File(all).All(ctx, &users); err != nil {
		panic(err)
	}
	for _, u := range users {
		fmt.Println(u.ID, u.Name)
	}

	// The same runner executes inside a transaction via tx.File.
	tx, err := db.Begin(ctx)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.File(insert).Exec(ctx, "carol"); err != nil {
		panic(err)
	}
	if err := tx.Commit(ctx); err != nil {
		panic(err)
	}

	var after []embedUser
	if err := db.File(all).All(ctx, &after); err != nil {
		panic(err)
	}
	fmt.Println("total:", len(after))
	// Output:
	// 1 alice
	// 2 bob
	// total: 3
}
