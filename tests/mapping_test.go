package tests

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mineot/krios"
)

type mappedUser struct {
	ID   int64   `krios:"id,pk"`
	Name string  `krios:"name"`
	Bio  *string `krios:"bio"`
}

func (mappedUser) TableName() string { return "users" }

type requiredBioUser struct {
	ID   int64  `krios:"id"`
	Name string `krios:"name"`
	Bio  string `krios:"bio"`
}

func (requiredBioUser) TableName() string { return "users" }

func seedUsers(t *testing.T) *krios.DB {
	t.Helper()

	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)"); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (1, 'alice', 'reader')"); err != nil {
		t.Fatalf("INSERT alice = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (2, 'bob', NULL)"); err != nil {
		t.Fatalf("INSERT bob = %v", err)
	}
	return db
}

func TestEntityOfPublicDescriptor(t *testing.T) {
	t.Parallel()

	ent, err := krios.EntityOf[mappedUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	if ent.Table() != "users" {
		t.Errorf("Table() = %q, want users", ent.Table())
	}
	if got, want := ent.Columns(), []string{"id", "name", "bio"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("Columns() = %v, want %v", got, want)
	}
	if keys := ent.PrimaryKeys(); len(keys) != 1 || keys[0] != "id" {
		t.Errorf("PrimaryKeys() = %v, want [id]", keys)
	}

	if _, err := krios.EntityOf[time.Time](); !errors.Is(err, krios.ErrInvalidEntity) {
		t.Errorf("EntityOf[time.Time]() = %v, want ErrInvalidEntity", err)
	}
	if _, err := krios.EntityOf[*mappedUser](); err != nil {
		t.Errorf("EntityOf[*mappedUser]() = %v", err)
	}
}

func TestRowScanMappedSingleRecord(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedUsers(t)

	var user mappedUser
	err := db.QueryRowContext(ctx, "SELECT id, name, bio FROM users WHERE id = ?", 1).Scan(&user)
	if err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if user.ID != 1 || user.Name != "alice" || user.Bio == nil || *user.Bio != "reader" {
		t.Errorf("user = %+v", user)
	}

	missing := mappedUser{}
	err = db.QueryRowContext(ctx, "SELECT id, name, bio FROM users WHERE id = ?", 99).Scan(&missing)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Scan() = %v, want sql.ErrNoRows", err)
	}
}

func TestRowsAllMappedRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedUsers(t)

	rows, err := db.QueryContext(ctx, "SELECT id, name, bio FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	defer rows.Close()

	var users []mappedUser
	if err := rows.All(&users); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}
	if users[0].ID != 1 || users[0].Name != "alice" || users[0].Bio == nil || *users[0].Bio != "reader" {
		t.Errorf("users[0] = %+v", users[0])
	}
	if users[1].ID != 2 || users[1].Name != "bob" || users[1].Bio != nil {
		t.Errorf("users[1] = %+v", users[1])
	}
}

func TestRowsAllAliasesAndSubset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedUsers(t)

	t.Run("alias and case-insensitive match", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, "SELECT id AS ID, name FROM users ORDER BY id")
		if err != nil {
			t.Fatalf("QueryContext() = %v", err)
		}
		defer rows.Close()

		var users []mappedUser
		if err := rows.All(&users); err != nil {
			t.Fatalf("All() = %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("len(users) = %d, want 2", len(users))
		}
		if users[1].ID != 2 || users[1].Name != "bob" || users[1].Bio != nil {
			t.Errorf("users[1] = %+v, want alias-mapped row with zero bio", users[1])
		}
	})

	t.Run("unselected columns keep zero values", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, "SELECT name FROM users ORDER BY id")
		if err != nil {
			t.Fatalf("QueryContext() = %v", err)
		}
		defer rows.Close()

		var users []mappedUser
		if err := rows.All(&users); err != nil {
			t.Fatalf("All() = %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("len(users) = %d, want 2", len(users))
		}
		if users[0].ID != 0 || users[0].Name != "alice" || users[0].Bio != nil {
			t.Errorf("users[0] = %+v, want only name filled", users[0])
		}
	})

	t.Run("unknown columns are discarded", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, "SELECT name, 'extra' AS nickname FROM users ORDER BY id")
		if err != nil {
			t.Fatalf("QueryContext() = %v", err)
		}
		defer rows.Close()

		var users []mappedUser
		if err := rows.All(&users); err != nil {
			t.Fatalf("All() = %v", err)
		}
		if len(users) != 2 || users[0].Name != "alice" {
			t.Fatalf("users = %+v", users)
		}
	})
}

func TestMappedNullAndConversionErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedUsers(t)

	t.Run("null into non-nullable", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, "SELECT id, name, bio FROM users ORDER BY id")
		if err != nil {
			t.Fatalf("QueryContext() = %v", err)
		}
		defer rows.Close()

		var users []requiredBioUser
		err = rows.All(&users)
		if err == nil || errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("All() = %v, want a conversion error", err)
		}
		if !strings.Contains(err.Error(), "users") {
			t.Errorf("All() = %v, want table context", err)
		}
	})

	t.Run("incompatible conversion", func(t *testing.T) {
		var user mappedUser
		err := db.QueryRowContext(ctx, "SELECT 'not an integer' AS id, name, bio FROM users WHERE id = 1").Scan(&user)
		if err == nil || errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("Scan() = %v, want a conversion error", err)
		}
		if !strings.Contains(err.Error(), "users") {
			t.Errorf("Scan() = %v, want table context", err)
		}
	})
}

func TestMappedReadFailureBlocksCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedUsers(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(ctx)

	var user mappedUser
	scanErr := tx.QueryRowContext(ctx, "SELECT 'not an integer' AS id, name, bio FROM users WHERE id = 1").Scan(&user)
	if scanErr == nil {
		t.Fatal("Scan() = nil, want conversion error")
	}

	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Fatal("Commit() = nil, want blocked commit")
	}
	if !strings.Contains(commitErr.Error(), "blocked") {
		t.Errorf("Commit() = %v, want blocked commit message", commitErr)
	}
	if !errors.Is(commitErr, scanErr) {
		t.Errorf("Commit() = %v, want the blocked read failure %v", commitErr, scanErr)
	}
}
