package tests

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/mineot/krios"
)

type writeAccount struct {
	ID   int64          `krios:"id,pk"`
	Name string         `krios:"name"`
	Bio  *string        `krios:"bio"`
	Age  sql.NullInt64  `krios:"age"`
}

func (writeAccount) TableName() string { return "write_accounts" }

func seedWriteDB(t *testing.T) *krios.DB {
	t.Helper()

	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE write_accounts (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		bio TEXT,
		age INTEGER
	)`); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}
	return db
}

func TestInsertReturnsResult(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	res, err := db.Insert(ent).
		SetValues(&writeAccount{ID: 1, Name: "alice", Bio: strptr("reader")}).
		Exec(ctx)
	if err != nil {
		t.Fatalf("Insert.Exec() = %v", err)
	}
	if n, ok := res.RowsAffected(); !ok || n != 1 {
		t.Errorf("RowsAffected() = %d, %v; want 1, true", n, ok)
	}

	if n := countRows(t, db, "write_accounts"); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

func TestInsertReturningMapsNewRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	var got writeAccount
	err = db.Insert(ent).
		SetValues(&writeAccount{ID: 7, Name: "bianca"}).
		Returning().
		One(ctx, &got)
	if err != nil {
		t.Fatalf("Insert.One() = %v", err)
	}
	if got.ID != 7 || got.Name != "bianca" || got.Bio != nil {
		t.Errorf("returned = %+v", got)
	}
}

func TestUpdateNilPointerLeavesColumnUnchanged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	if _, err := db.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "alice", Bio: strptr("reader")}).Exec(ctx); err != nil {
		t.Fatalf("seed Insert = %v", err)
	}

	// Bio nil: not updated. Name non-nullable writes always.
	res, err := db.Update(ent).
		SetValues(&writeAccount{ID: 1, Name: "alice2", Bio: nil}).
		Where(ent.Col("id").Eq(1)).
		Exec(ctx)
	if err != nil {
		t.Fatalf("Update.Exec() = %v", err)
	}
	if n, ok := res.RowsAffected(); !ok || n != 1 {
		t.Errorf("RowsAffected() = %d, %v; want 1, true", n, ok)
	}

	var bio sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT bio FROM write_accounts WHERE id = 1").Scan(&bio); err != nil {
		t.Fatalf("SELECT bio = %v", err)
	}
	if !bio.Valid || bio.String != "reader" {
		t.Errorf("bio = %+v, want unchanged reader", bio)
	}

	var name string
	if err := db.QueryRowContext(ctx, "SELECT name FROM write_accounts WHERE id = 1").Scan(&name); err != nil {
		t.Fatalf("SELECT name = %v", err)
	}
	if name != "alice2" {
		t.Errorf("name = %q, want alice2", name)
	}
}

func TestUpdateReturningMapsRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	for i := int64(1); i <= 2; i++ {
		if _, err := db.Insert(ent).SetValues(&writeAccount{ID: i, Name: "u"}).Exec(ctx); err != nil {
			t.Fatalf("seed Insert = %v", err)
		}
	}

	var got []writeAccount
	err = db.Update(ent).
		SetValues(&writeAccount{Name: "renamed"}).
		Omit("id", "bio", "age").
		Where(ent.Col("id").In(int64(1), int64(2))).
		Returning().
		All(ctx, &got)
	if err != nil {
		t.Fatalf("Update.All() = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("returned %d rows, want 2", len(got))
	}
	for _, row := range got {
		if row.Name != "renamed" {
			t.Errorf("returned name = %q, want renamed", row.Name)
		}
	}
}

func TestDeleteRequiresCondition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	_, err = db.Delete(ent).Exec(ctx)
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Delete without Where = %v, want ErrInvalidQuery", err)
	}
	if n := countRows(t, db, "write_accounts"); n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func TestDeleteReturningMapsRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	if _, err := db.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "gone"}).Exec(ctx); err != nil {
		t.Fatalf("seed Insert = %v", err)
	}

	var got []writeAccount
	err = db.Delete(ent).
		Where(ent.Col("id").Eq(1)).
		Returning().
		All(ctx, &got)
	if err != nil {
		t.Fatalf("Delete.All() = %v", err)
	}
	if len(got) != 1 || got[0].Name != "gone" {
		t.Errorf("returned = %+v", got)
	}
	if n := countRows(t, db, "write_accounts"); n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func TestWriteInsideTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "tx"}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert = %v", err)
	}
		if _, err := tx.Update(ent).SetValues(&writeAccount{Name: "tx2"}).Omit("id", "bio", "age").Where(ent.Col("id").Eq(1)).Exec(ctx); err != nil {
			t.Fatalf("tx Update = %v", err)
		}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}

	var name string
	if err := db.QueryRowContext(ctx, "SELECT name FROM write_accounts WHERE id = 1").Scan(&name); err != nil {
		t.Fatalf("SELECT name = %v", err)
	}
	if name != "tx2" {
		t.Errorf("name = %q, want tx2", name)
	}
}

func TestWriteConstructionErrorDoesNotCloseTx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(context.Background())

	// Construction failure: no condition; must not reach the driver nor block Commit.
	if _, err := tx.Delete(ent).Exec(ctx); !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("tx Delete without Where = %v, want ErrInvalidQuery", err)
	}
	if _, err := tx.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "ok"}).Exec(ctx); err != nil {
		t.Fatalf("tx Insert after construction error = %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v, want nil", err)
	}
}

func TestWriteValidationFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[checkedWriteAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	_, err = db.Insert(ent).SetValues(&checkedWriteAccount{Name: "  "}).Exec(ctx)
	if !errors.Is(err, krios.ErrValidation) {
		t.Fatalf("Insert with invalid field = %v, want ErrValidation", err)
	}
}

func TestUpdateSetValuesWrongType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	_, err = db.Update(ent).
		SetValues(&mappedUser{}).
		Where(ent.Col("id").Eq(1)).
		Exec(ctx)
	if !errors.Is(err, krios.ErrInvalidEntity) {
		t.Fatalf("Update wrong type = %v, want ErrInvalidEntity", err)
	}
}

func TestInsertOmitUsesDatabaseDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	var got writeAccount
	err = db.Insert(ent).
		SetValues(&writeAccount{Name: "gen"}).
		Omit("id").
		Returning().
		One(ctx, &got)
	if err != nil {
		t.Fatalf("Insert.One() = %v", err)
	}
	if got.Name != "gen" {
		t.Errorf("returned name = %q, want gen", got.Name)
	}
}

type checkedWriteAccount struct {
	ID   int64        `krios:"id,pk"`
	Name checkedTitle `krios:"name"`
}

func (checkedWriteAccount) TableName() string { return "write_accounts" }

type checkedTitle string

func (c checkedTitle) Validate() error {
	if strings.TrimSpace(string(c)) == "" {
		return errors.New("name is required")
	}
	return nil
}

func strptr(s string) *string { return &s }

func TestInsertMultipleRowsAffected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	for i := int64(1); i <= 3; i++ {
		res, err := db.Insert(ent).SetValues(&writeAccount{ID: i, Name: "u"}).Exec(ctx)
		if err != nil {
			t.Fatalf("Insert.Exec() #%d = %v", i, err)
		}
		if n, ok := res.RowsAffected(); !ok || n != 1 {
			t.Fatalf("Insert.RowsAffected() #%d = %d, %v; want 1, true", i, n, ok)
		}
	}
	if n := countRows(t, db, "write_accounts"); n != 3 {
		t.Fatalf("rows = %d, want 3", n)
	}
}

func TestUpdateMultipleRowsAffected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	for i := int64(1); i <= 3; i++ {
		if _, err := db.Insert(ent).SetValues(&writeAccount{ID: i, Name: "old"}).Exec(ctx); err != nil {
			t.Fatalf("seed Insert = %v", err)
		}
	}

	res, err := db.Update(ent).
		SetValues(&writeAccount{Name: "new"}).
		Omit("id", "bio", "age").
		Where(ent.Col("id").In(int64(1), int64(2), int64(3))).
		Exec(ctx)
	if err != nil {
		t.Fatalf("Update.Exec() = %v", err)
	}
	if n, ok := res.RowsAffected(); !ok || n != 3 {
		t.Fatalf("RowsAffected() = %d, %v; want 3, true", n, ok)
	}
}

func TestDeleteMultipleRowsAffected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	for i := int64(1); i <= 2; i++ {
		if _, err := db.Insert(ent).SetValues(&writeAccount{ID: i, Name: "x"}).Exec(ctx); err != nil {
			t.Fatalf("seed Insert = %v", err)
		}
	}

	res, err := db.Delete(ent).Where(ent.Col("id").In(int64(1), int64(2))).Exec(ctx)
	if err != nil {
		t.Fatalf("Delete.Exec() = %v", err)
	}
	if n, ok := res.RowsAffected(); !ok || n != 2 {
		t.Fatalf("RowsAffected() = %d, %v; want 2, true", n, ok)
	}
	if n := countRows(t, db, "write_accounts"); n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func TestInsertNilPointerWritesNull(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	if _, err := db.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "n", Bio: nil}).Exec(ctx); err != nil {
		t.Fatalf("Insert.Exec() = %v", err)
	}

	var bio sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT bio FROM write_accounts WHERE id = 1").Scan(&bio); err != nil {
		t.Fatalf("SELECT bio = %v", err)
	}
	if bio.Valid {
		t.Errorf("bio = %+v, want NULL", bio)
	}
}

func TestUpdateDoesNotUpdateNilPointerColumn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	if _, err := db.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "a", Bio: strptr("keep")}).Exec(ctx); err != nil {
		t.Fatalf("seed = %v", err)
	}

	if _, err := db.Update(ent).SetValues(&writeAccount{Name: "b", Bio: nil}).Omit("id", "age").Where(ent.Col("id").Eq(1)).Exec(ctx); err != nil {
		t.Fatalf("Update.Exec() = %v", err)
	}

	var bio sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT bio FROM write_accounts WHERE id = 1").Scan(&bio); err != nil {
		t.Fatalf("SELECT bio = %v", err)
	}
	if !bio.Valid || bio.String != "keep" {
		t.Errorf("bio = %+v, want keep", bio)
	}
}

func TestUpdateEmptyWhereIsRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	_, err = db.Update(ent).SetValues(&writeAccount{Name: "x"}).Where().Exec(ctx)
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Update.Where() empty = %v, want ErrInvalidQuery", err)
	}
}

func TestDeleteEmptyWhereIsRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	_, err = db.Delete(ent).Where().Exec(ctx)
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Delete.Where() empty = %v, want ErrInvalidQuery", err)
	}
}

func TestReturningExecRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	_, err = db.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "x"}).Returning().Exec(ctx)
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Insert.Exec() with Returning = %v, want ErrInvalidQuery", err)
	}

	_, err = db.Update(ent).SetValues(&writeAccount{Name: "x"}).Omit("id", "bio", "age").Where(ent.Col("id").Eq(1)).Returning().Exec(ctx)
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Update.Exec() with Returning = %v, want ErrInvalidQuery", err)
	}

	_, err = db.Delete(ent).Where(ent.Col("id").Eq(1)).Returning().Exec(ctx)
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Delete.Exec() with Returning = %v, want ErrInvalidQuery", err)
	}
}

func TestAllOneWithoutReturningRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedWriteDB(t)
	ent, err := krios.EntityOf[writeAccount]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	if _, err := db.Insert(ent).SetValues(&writeAccount{ID: 1, Name: "x"}).Exec(ctx); err != nil {
		t.Fatalf("seed = %v", err)
	}

	err = db.Update(ent).SetValues(&writeAccount{Name: "y"}).Omit("id", "bio", "age").Where(ent.Col("id").Eq(1)).All(ctx, &[]writeAccount{})
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Update.All() without Returning = %v, want ErrInvalidQuery", err)
	}

	err = db.Delete(ent).Where(ent.Col("id").Eq(1)).All(ctx, &[]writeAccount{})
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Delete.All() without Returning = %v, want ErrInvalidQuery", err)
	}

	err = db.Insert(ent).SetValues(&writeAccount{Name: "z"}).One(ctx, &writeAccount{})
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Insert.One() without Returning = %v, want ErrInvalidQuery", err)
	}
}
