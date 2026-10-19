package tests

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/mineot/krios"
)

// orderLabel maps a differently-labeled result set; Row.Scan is positional, so
// only the declaration order of the fields matters.
type orderLabel struct {
	ID   int64  `krios:"id"`
	Name string `krios:"name"`
}

func (orderLabel) TableName() string { return "labels" }

// nullUser exercises sql.Null* fields next to a pointer field.
type nullUser struct {
	ID   int64          `krios:"id"`
	Name string         `krios:"name"`
	Bio  sql.NullString `krios:"bio"`
	Age  sql.NullInt64  `krios:"age"`
}

func (nullUser) TableName() string { return "null_users" }

func seedNullUsers(t *testing.T) *krios.DB {
	t.Helper()

	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE null_users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		bio TEXT,
		age INTEGER
	)`); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO null_users (id, name, bio, age) VALUES (1, 'alice', 'reader', 30), (2, 'bob', NULL, NULL)"); err != nil {
		t.Fatalf("INSERT = %v", err)
	}
	return db
}

func TestMappingPositionalRowScanIgnoresColumnNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)

	if _, err := db.ExecContext(ctx, "CREATE TABLE labels (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatalf("CREATE TABLE = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO labels (id, name) VALUES (7, 'alice')"); err != nil {
		t.Fatalf("INSERT = %v", err)
	}

	// columns are aliased to unrelated labels; positional Scan still maps by
	// declaration order (id, name).
	var got orderLabel
	err := db.QueryRowContext(ctx, "SELECT id AS first, name AS second FROM labels").Scan(&got)
	if err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if got.ID != 7 || got.Name != "alice" {
		t.Errorf("Scan() = %+v, want positional fields id=7 name=alice", got)
	}
}

func TestMappingMultipleRecordsStayIndependent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedAccounts(t)

	rows, err := db.QueryContext(ctx, "SELECT id, email, status, bio FROM accounts ORDER BY id DESC")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	defer rows.Close()

	var accounts []account
	if err := rows.All(&accounts); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("len(accounts) = %d, want 2", len(accounts))
	}
	if accounts[0].ID != 2 || accounts[0].Email != "bob@example.com" || accounts[0].Bio != nil {
		t.Errorf("accounts[0] = %+v, want the second row with nil bio", accounts[0])
	}
	if accounts[1].ID != 1 || accounts[1].Email != "alice@example.com" || accounts[1].Bio == nil || *accounts[1].Bio != "reader" {
		t.Errorf("accounts[1] = %+v, want the first row with its own bio", accounts[1])
	}
}

func TestMappingAliasesColumnAndTable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedAccounts(t)

	rows, err := db.QueryContext(ctx, `SELECT a.id, a.email AS mail, a.status, a.bio FROM accounts AS a ORDER BY a.id`)
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	defer rows.Close()

	// "mail" has no mapped field: it is discarded, and email stays zero.
	var accounts []account
	if err := rows.All(&accounts); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("len(accounts) = %d, want 2", len(accounts))
	}
	if accounts[0].Email != "" || accounts[0].Status != "active" {
		t.Errorf("accounts[0] = %+v, want email zero and status mapped", accounts[0])
	}
}

func TestMappingNullTypes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedNullUsers(t)

	rows, err := db.QueryContext(ctx, "SELECT id, name, bio, age FROM null_users ORDER BY id")
	if err != nil {
		t.Fatalf("QueryContext() = %v", err)
	}
	defer rows.Close()

	var users []nullUser
	if err := rows.All(&users); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}
	if !users[0].Bio.Valid || users[0].Bio.String != "reader" || !users[0].Age.Valid || users[0].Age.Int64 != 30 {
		t.Errorf("users[0] = %+v, want valid bio and age", users[0])
	}
	if users[1].Bio.Valid || users[1].Age.Valid {
		t.Errorf("users[1] = %+v, want both NULL fields invalid", users[1])
	}

	ent, err := krios.EntityOf[nullUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	formatted, err := ent.Format(users[0])
	if err != nil {
		t.Fatalf("Format() = %v", err)
	}
	// sql.Null* without a Formatter falls back to fmt.Sprint of the struct.
	if !strings.Contains(formatted["bio"], "reader") {
		t.Errorf("Format()[bio] = %q, want the NullString internals", formatted["bio"])
	}
	if formatted["id"] != "1" {
		t.Errorf("Format()[id] = %q, want 1", formatted["id"])
	}
}

func TestMappingValidationStopsAtFirstInvalidRecord(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedAccounts(t)

	if _, err := db.ExecContext(ctx, "INSERT INTO accounts (id, email, status, bio) VALUES (3, 'alice@example.com', 'active', NULL), (4, 'bad-email', 'active', NULL)"); err != nil {
		t.Fatalf("INSERT = %v", err)
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

func TestMappingFormattingIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedAccounts(t)

	var acct account
	if err := db.QueryRowContext(ctx, "SELECT id, email, status, bio FROM accounts WHERE id = 1").Scan(&acct); err != nil {
		t.Fatalf("Scan() = %v", err)
	}

	ent, err := krios.EntityOf[account]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}
	formatted, err := ent.Format(acct)
	if err != nil {
		t.Fatalf("Format() = %v", err)
	}
	want := map[string]string{
		"id":     "1",
		"email":  "alice@example.com",
		"status": "ACTIVE",
		"bio":    "reader",
	}
	for column, text := range want {
		if formatted[column] != text {
			t.Errorf("Format()[%q] = %q, want %q", column, formatted[column], text)
		}
	}
	if len(formatted) != len(want) {
		t.Errorf("Format() returned %d columns, want %d", len(formatted), len(want))
	}
}
