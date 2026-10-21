// Consumer for the C06-P01 manual scenarios. No server is provisioned here.
package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

//go:embed sql/*/*.sql
var files embed.FS

type state string

func (s state) Validate() error {
	if s != "valid" {
		return fmt.Errorf("invalid state %q", s)
	}
	return nil
}

type entry struct {
	ID       int64          `krios:"id,pk"`
	Name     string         `krios:"name"`
	Quantity int64          `krios:"quantity"`
	Enabled  bool           `krios:"enabled"`
	Note     *string        `krios:"note"`
	Nullable sql.NullString `krios:"nullable"`
	State    state          `krios:"state"`
}

func (entry) TableName() string { return "krios_c06_entries" }

func main() {
	engine := flag.String("db", "sqlite", "sqlite, mysql or postgres")
	flag.Parse()
	cfg := krios.Config{Type: krios.DBType(*engine), Timeout: 10 * time.Second}
	switch *engine {
	case "sqlite":
		dir, err := os.MkdirTemp("", "krios-c06-")
		must(err)
		defer os.RemoveAll(dir)
		cfg.Driver = "sqlite"
		cfg.SQLite = &krios.SQLiteConfig{DBPath: filepath.Join(dir, "check.db")}
	case "mysql":
		cfg.Driver = "mysql"
		cfg.MySQL = &krios.MySQLConfig{Host: os.Getenv("KRIOS_TEST_HOST"), User: os.Getenv("KRIOS_TEST_USER"), Password: os.Getenv("KRIOS_TEST_PASSWORD"), DBName: os.Getenv("KRIOS_TEST_DATABASE")}
	case "postgres":
		cfg.Driver = "pgx"
		cfg.Postgres = &krios.PostgresConfig{Host: os.Getenv("KRIOS_TEST_HOST"), User: os.Getenv("KRIOS_TEST_USER"), Password: os.Getenv("KRIOS_TEST_PASSWORD"), DBName: os.Getenv("KRIOS_TEST_DATABASE")}
	default:
		panic("unsupported db")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	must(run(ctx, cfg))
	fmt.Println("C06-P01 PASS", *engine)
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func check(ok bool, format string, args ...any) {
	if !ok {
		panic(fmt.Sprintf(format, args...))
	}
}
func wantErr(err, target error) { check(errors.Is(err, target), "error=%v, want %v", err, target) }
func resource(engine, name string) *krios.SQLFile {
	f, err := krios.NewSQLFile(files, "sql/"+engine+"/"+name+".sql")
	must(err)
	return f
}
func result(r krios.Result) {
	n, ok := r.RowsAffected()
	fmt.Printf("RowsAffected=%d available=%t\n", n, ok)
	id, ok := r.LastInsertId()
	fmt.Printf("LastInsertId=%d available=%t\n", id, ok)
}

func run(ctx context.Context, cfg krios.Config) (retErr error) {
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	engine := string(cfg.Type)
	// Ownership is acquired only after a successful CREATE, without IF NOT EXISTS.
	if _, err = db.File(resource(engine, "create")).Exec(ctx); err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_, err := db.ExecContext(cleanup, "DROP TABLE krios_c06_entries")
		if err != nil {
			fmt.Fprintln(os.Stderr, "cleanup failed for krios_c06_entries:", err)
		}
		retErr = errors.Join(retErr, err)
	}()
	ent, err := krios.EntityOf[entry]()
	if err != nil {
		return err
	}
	count := func() int64 { n, err := db.Select(ent).Count(ctx); must(err); return n }
	read := func(id int64) entry {
		var v entry
		must(db.Select(ent).Where(ent.Col("id").Eq(id)).One(ctx, &v))
		return v
	}
	snapshot := func() []entry {
		var v []entry
		must(db.Select(ent).OrderBy(ent.Col("id").Asc()).All(ctx, &v))
		return v
	}
	unchanged := func(before []entry) { check(reflect.DeepEqual(before, snapshot()), "unexpected persisted effect") }

	// W01: explicit zeros, nil INSERT, database defaults through Omit.
	r, err := db.Insert(ent).SetValues(entry{ID: 1, Name: "zero", State: "valid"}).Exec(ctx)
	must(err)
	result(r)
	a := read(1)
	check(a.Quantity == 0 && !a.Enabled && a.Note == nil && !a.Nullable.Valid, "W01 zeros/NULL: %+v", a)
	_, err = db.Insert(ent).SetValues(entry{ID: 2, Name: "default", State: "valid"}).Omit("quantity", "enabled", "note", "nullable").Exec(ctx)
	must(err)
	a = read(2)
	check(a.Quantity == 7 && a.Enabled && a.Note != nil && *a.Note == "default" && a.Nullable.Valid && a.Nullable.String == "default", "W01 defaults: %+v", a)
	fmt.Println("W01 PASS")

	// W02: nil pointer preserves UPDATE; invalid NullString writes NULL.
	_, err = db.Update(ent).SetValues(entry{ID: 2, Name: "updated", State: "valid"}).Omit("id").Where(ent.Col("id").Eq(2)).Exec(ctx)
	must(err)
	a = read(2)
	check(a.Quantity == 0 && !a.Enabled && a.Note != nil && *a.Note == "default" && !a.Nullable.Valid, "W02 values: %+v", a)
	fmt.Println("W02 PASS")

	// W03: invalid values / missing filters never change data, even in Tx.
	before := snapshot()
	tx, err := db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.Insert(ent).SetValues(entry{ID: 3, State: "invalid"}).Exec(ctx)
	wantErr(err, krios.ErrValidation)
	_, err = tx.Update(ent).SetValues(entry{State: "valid"}).Exec(ctx)
	wantErr(err, krios.ErrInvalidQuery)
	_, err = tx.Delete(ent).Exec(ctx)
	wantErr(err, krios.ErrInvalidQuery)
	must(tx.Commit(ctx))
	unchanged(before)
	fmt.Println("W03 PASS")

	// W04: Returning is rejected without effects on MySQL; returned rows elsewhere.
	before = snapshot()
	var inserted entry
	err = db.Insert(ent).SetValues(entry{ID: 3, Name: "returned", State: "valid"}).Returning().One(ctx, &inserted)
	if cfg.Type == krios.DBTypeMySQL {
		wantErr(err, krios.ErrInvalidQuery)
		var rows []entry
		err = db.Update(ent).SetValues(entry{State: "valid"}).Omit("id").Where(ent.Col("id").Gt(0)).Returning().All(ctx, &rows)
		wantErr(err, krios.ErrInvalidQuery)
		err = db.Delete(ent).Where(ent.Col("id").Gt(0)).Returning().All(ctx, &rows)
		wantErr(err, krios.ErrInvalidQuery)
		unchanged(before)
	} else {
		must(err)
		check(inserted.ID == 3 && inserted.Name == "returned", "W04 insert: %+v", inserted)
		var rows []entry
		must(db.Update(ent).SetValues(entry{Name: "changed", State: "valid"}).Omit("id").Where(ent.Col("id").Eq(3)).Returning().All(ctx, &rows))
		check(len(rows) == 1 && rows[0].Name == "changed" && read(3).Name == "changed", "W04 update: %+v", rows)
		must(db.Delete(ent).Where(ent.Col("id").Eq(3)).Returning().All(ctx, &rows))
		check(len(rows) == 1 && rows[0].ID == 3, "W04 delete: %+v", rows)
		unchanged(before)
	}
	fmt.Println("W04 PASS")

	// W05: committed writes and rolled-back writes checked through DB.
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.Insert(ent).SetValues(entry{ID: 4, Name: "commit", State: "valid"}).Exec(ctx)
	must(err)
	must(tx.Commit(ctx))
	check(read(4).Name == "commit", "W05 commit")
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.Delete(ent).Where(ent.Col("id").Eq(4)).Exec(ctx)
	must(err)
	must(tx.Rollback(ctx))
	check(read(4).Name == "commit", "W05 rollback")
	_, err = db.Delete(ent).Where(ent.Col("id").Eq(4)).Exec(ctx)
	must(err)
	fmt.Println("W05 PASS")

	// W06: multiple rows, checking persisted effects without assuming Result flags.
	r, err = db.Update(ent).SetValues(entry{Name: "multi", State: "valid"}).Omit("id").Where(ent.Col("id").Gt(0)).Exec(ctx)
	must(err)
	result(r)
	check(read(1).Name == "multi" && read(2).Name == "multi", "W06 multi update")
	fmt.Println("W06 update PASS")

	// F01: actual embedded SQL, arguments passed separately, One positional / All by name.
	r, err = db.File(resource(engine, "insert")).Exec(ctx, 10, "file")
	must(err)
	result(r)
	type pair struct {
		ID   int64  `krios:"id"`
		Name string `krios:"name"`
	}
	var one pair
	must(db.File(resource(engine, "one")).One(ctx, &one, 10))
	check(one.ID == 10 && one.Name == "file", "F01 One: %+v", one)
	var all []pair
	must(db.File(resource(engine, "all")).All(ctx, &all, 10))
	check(len(all) == 1 && all[0] == one, "F01 All: %+v", all)
	fmt.Println("F01 PASS")

	// F02: loading failures and nil runner do not poison a Tx.
	before = snapshot()
	_, err = krios.NewSQLFile(files, "sql/missing.sql")
	wantErr(err, krios.ErrFileNotFound)
	_, err = krios.NewSQLFile(files, "sql/"+engine+"/blank.sql")
	wantErr(err, krios.ErrFileNotFound)
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.File(nil).Exec(ctx)
	wantErr(err, krios.ErrInvalidQuery)
	must(tx.Commit(ctx))
	unchanged(before)
	fmt.Println("F02 PASS")

	// F03: file writes and reads inside Tx; rollback then commit are observed outside.
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.File(resource(engine, "insert")).Exec(ctx, 11, "rollback")
	must(err)
	must(tx.File(resource(engine, "one")).One(ctx, &one, 11))
	check(one.ID == 11, "F03 One")
	must(tx.File(resource(engine, "all")).All(ctx, &all, 11))
	check(len(all) == 1, "F03 All")
	must(tx.Rollback(ctx))
	unchanged(before)
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.File(resource(engine, "insert")).Exec(ctx, 11, "committed")
	must(err)
	must(tx.Commit(ctx))
	check(read(11).Name == "committed", "F03 commit")
	fmt.Println("F03 PASS")

	// F04: late conversion error must block Commit and discard previous write.
	before = snapshot()
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.File(resource(engine, "insert")).Exec(ctx, 12, "discard")
	must(err)
	var number int64
	scanErr := tx.File(resource(engine, "badscan")).QueryRow(ctx).Scan(&number)
	check(scanErr != nil, "F04 expected conversion failure")
	err = tx.Commit(ctx)
	check(errors.Is(err, scanErr), "F04 commit=%v scan=%v", err, scanErr)
	unchanged(before)
	// sql.ErrNoRows does not block Commit.
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	err = tx.File(resource(engine, "one")).One(ctx, &one, 999)
	wantErr(err, sql.ErrNoRows)
	must(tx.Commit(ctx))
	fmt.Println("F04 PASS")

	// F05: SQL execution failure (distinct from loading and conversion failures).
	before = snapshot()
	tx, err = db.Begin(ctx)
	must(err)
	defer tx.Rollback(context.Background())
	_, err = tx.File(resource(engine, "insert")).Exec(ctx, 13, "discard")
	must(err)
	_, bodyErr := tx.File(resource(engine, "invalid")).Exec(ctx)
	check(bodyErr != nil, "F05 invalid SQL succeeded")
	err = tx.Commit(ctx)
	check(errors.Is(err, bodyErr), "F05 lost SQL error: %v", err)
	unchanged(before)
	fmt.Println("F05 PASS")

	r, err = db.Delete(ent).Where(ent.Col("id").Gt(0)).Exec(ctx)
	must(err)
	result(r)
	check(count() == 0, "W06 multi delete")
	fmt.Println("W06 delete PASS")
	// W07: generated key; availability flags are observed, not assumed.
	r, err = db.Insert(ent).SetValues(entry{Name: "generated", State: "valid"}).Omit("id").Exec(ctx)
	must(err)
	result(r)
	var generated entry
	must(db.Select(ent).Where(ent.Col("name").Eq("generated")).One(ctx, &generated))
	check(generated.ID > 0, "W07 generated ID: %d", generated.ID)
	if id, ok := r.LastInsertId(); ok {
		check(id == generated.ID, "W07 result id=%d persisted=%d", id, generated.ID)
	}
	_, err = db.Delete(ent).Where(ent.Col("id").Eq(generated.ID)).Exec(ctx)
	must(err)
	check(count() == 0, "W07 cleanup")
	fmt.Println("W07 PASS")
	return nil
}
