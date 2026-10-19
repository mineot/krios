package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mineot/krios"
)

type accountStatus string

func (s accountStatus) Validate() error {
	switch s {
	case "active", "suspended":
		return nil
	default:
		return fmt.Errorf("unknown status %q", string(s))
	}
}

func (s accountStatus) Format() (string, error) {
	return strings.ToUpper(string(s)), nil
}

type account struct {
	ID     int64         `krios:"id,pk"`
	Email  accountEmail  `krios:"email"`
	Status accountStatus `krios:"status"`
	Bio    *string       `krios:"bio"`
}

func (account) TableName() string { return "accounts" }

type accountEmail string

func (e accountEmail) Validate() error {
	if !strings.Contains(string(e), "@") {
		return errors.New("email must contain @")
	}
	return nil
}

func (e accountEmail) Format() (string, error) {
	return string(e), nil
}

func seedAccounts(t *testing.T) *krios.DB {
	t.Helper()

	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE accounts (
		id INTEGER PRIMARY KEY,
		email TEXT NOT NULL,
		status TEXT NOT NULL,
		bio TEXT
	)`); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO accounts (id, email, status, bio) VALUES (1, 'alice@example.com', 'active', 'reader'), (2, 'bob@example.com', 'suspended', NULL)"); err != nil {
		t.Fatalf("INSERT = %v", err)
	}
	return db
}

func TestEntityValidationAfterMappedRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedAccounts(t)

	rows, err := db.QueryContext(ctx, "SELECT id, email, status, bio FROM accounts ORDER BY id")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}

	var accounts []account
	if err := rows.All(&accounts); err != nil {
		t.Fatalf("All() = %v", err)
	}

	ent, err := krios.EntityOf[account]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	if err := ent.ValidateAll(accounts); err != nil {
		t.Fatalf("ValidateAll() = %v, want valid records", err)
	}

	formatted, err := ent.Format(accounts[0])
	if err != nil {
		t.Fatalf("Format() = %v", err)
	}
	if formatted["status"] != "ACTIVE" || formatted["email"] != "alice@example.com" {
		t.Errorf("Format() = %v", formatted)
	}
	if formatted["bio"] != "reader" {
		t.Errorf("Format()[bio] = %q, want reader", formatted["bio"])
	}
	if formatted["id"] != "1" {
		t.Errorf("Format()[id] = %q, want 1 (fallback)", formatted["id"])
	}

	nilBio, err := ent.Format(accounts[1])
	if err != nil {
		t.Fatalf("Format() = %v", err)
	}
	if nilBio["bio"] != "" {
		t.Errorf("Format()[bio] for nil pointer = %q, want empty", nilBio["bio"])
	}
}

func TestEntityValidationReportsInvalidRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedAccounts(t)

	if _, err := db.ExecContext(ctx, "INSERT INTO accounts (id, email, status, bio) VALUES (3, 'no-at-sign', 'active', NULL)"); err != nil {
		t.Fatalf("INSERT invalid = %v", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT id, email, status, bio FROM accounts ORDER BY id")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	var accounts []account
	if err := rows.All(&accounts); err != nil {
		t.Fatalf("All() = %v", err)
	}

	ent, err := krios.EntityOf[account]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	err = ent.ValidateAll(accounts)
	if !errors.Is(err, krios.ErrValidation) {
		t.Fatalf("ValidateAll() = %v, want ErrValidation", err)
	}
	if !strings.Contains(err.Error(), `column "email"`) || !strings.Contains(err.Error(), "accounts") {
		t.Errorf("ValidateAll() = %v, want table and column context", err)
	}
}
