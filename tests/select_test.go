package tests

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/mineot/krios"
)

type selectUser struct {
	ID   int64   `krios:"id,pk"`
	Name string  `krios:"name"`
	Bio  *string `krios:"bio"`
}

func (selectUser) TableName() string { return "users" }

type selectTask struct {
	ID      int64  `krios:"id,pk"`
	OwnerID int64  `krios:"owner_id"`
	Title   string `krios:"title"`
}

func (selectTask) TableName() string { return "tasks" }

// seedSelectDB creates users and tasks with a user (carol) that owns no task
// and a task-free user for LEFT JOIN coverage.
func seedSelectDB(t *testing.T) *krios.DB {
	t.Helper()

	db := seedUsers(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, "CREATE TABLE tasks (id INTEGER PRIMARY KEY, owner_id INTEGER NOT NULL, title TEXT NOT NULL)"); err != nil {
		t.Fatalf("CREATE TABLE tasks = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO tasks (id, owner_id, title) VALUES (1, 1, 'write'), (2, 1, 'review'), (3, 2, 'deploy')"); err != nil {
		t.Fatalf("INSERT tasks = %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, bio) VALUES (3, 'carol', NULL)"); err != nil {
		t.Fatalf("INSERT carol = %v", err)
	}
	return db
}

func TestSelectAllAndFilters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	var all []selectUser
	if err := db.Select(users).All(ctx, &all); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("All() returned %d users, want 3", len(all))
	}

	var one selectUser
	err = db.Select(users).Where(users.Col("name").Eq("alice")).One(ctx, &one)
	if err != nil {
		t.Fatalf("One() = %v", err)
	}
	if one.ID != 1 || one.Name != "alice" {
		t.Errorf("One() = %+v, want alice", one)
	}

	var missing selectUser
	err = db.Select(users).Where(users.Col("name").Eq("nobody")).One(ctx, &missing)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("One() missing = %v, want sql.ErrNoRows", err)
	}

	var starts []selectUser
	if err := db.Select(users).Where(users.Col("name").Like(krios.StartsWith, "a")).All(ctx, &starts); err != nil {
		t.Fatalf("All() Like = %v", err)
	}
	if len(starts) != 1 || starts[0].Name != "alice" {
		t.Errorf("Like(StartsWith, a) = %v, want alice", starts)
	}

	var in []selectUser
	if err := db.Select(users).Where(users.Col("id").In(int64(1), int64(3))).All(ctx, &in); err != nil {
		t.Fatalf("All() In = %v", err)
	}
	if len(in) != 2 {
		t.Errorf("In(1, 3) returned %d users, want 2", len(in))
	}

	// Or(id = 2, And(bio IS NOT NULL, name <> 'bob')) => bob + alice.
	var logical []selectUser
	cond := krios.Or(
		users.Col("id").Eq(int64(2)),
		krios.And(users.Col("bio").IsNotNull(), users.Col("name").Ne("bob")),
	)
	if err := db.Select(users).Where(cond).All(ctx, &logical); err != nil {
		t.Fatalf("All() Or/And = %v", err)
	}
	if len(logical) != 2 {
		t.Errorf("logical filter returned %d users, want 2", len(logical))
	}
}

func TestSelectOnlyKeepsUnselectedFieldsZero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	var names []selectUser
	if err := db.Select(users).Only("name").All(ctx, &names); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("All() returned %d users, want 3", len(names))
	}
	for _, u := range names {
		if u.ID != 0 || u.Bio != nil || u.Name == "" {
			t.Errorf("Only(name) mapped %+v, want only Name filled", u)
		}
	}
}

func TestSelectJoins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}
	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}

	var aliceTasks []selectTask
	err = db.Select(tasks).
		Join(users, krios.InnerJoin, users.Col("id").Eq(tasks.Col("owner_id"))).
		Where(users.Col("name").Eq("alice")).
		All(ctx, &aliceTasks)
	if err != nil {
		t.Fatalf("All() inner join = %v", err)
	}
	if len(aliceTasks) != 2 {
		t.Errorf("inner join returned %d tasks, want 2", len(aliceTasks))
	}

	var withoutTasks []selectUser
	err = db.Select(users).
		Join(tasks, krios.LeftJoin, tasks.Col("owner_id").Eq(users.Col("id"))).
		Where(tasks.Col("id").IsNull()).
		All(ctx, &withoutTasks)
	if err != nil {
		t.Fatalf("All() left join = %v", err)
	}
	if len(withoutTasks) != 1 || withoutTasks[0].Name != "carol" {
		t.Errorf("left join without tasks = %v, want carol", withoutTasks)
	}
}

func TestSelectBuildDoesNotExecute(t *testing.T) {
	t.Parallel()

	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	query, args, err := db.Select(users).
		Only("id", "name").
		Where(users.Col("name").Like(krios.Contains, "li")).
		Build()
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	want := `SELECT "users"."id", "users"."name" FROM "users" WHERE "users"."name" LIKE ? ESCAPE '!'`
	if query != want {
		t.Errorf("Build() = %q, want %q", query, want)
	}
	if len(args) != 1 || args[0] != "%li%" {
		t.Errorf("Build() args = %v, want [%%li%%]", args)
	}
}

func TestSelectInvalidQueryIsReportedBeforeExecution(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	var result []selectUser
	err = db.Select(users).Where(users.Col("missing").Eq(1)).All(ctx, &result)
	if !errors.Is(err, krios.ErrInvalidQuery) {
		t.Errorf("All() = %v, want ErrInvalidQuery", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	err = db.Select(users).All(ctx, &result)
	if !errors.Is(err, krios.ErrClosed) {
		t.Errorf("All() after Close = %v, want ErrClosed", err)
	}
}

func TestSelectInsideTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var one selectUser
	if err := tx.Select(users).Where(users.Col("name").Eq("bob")).One(ctx, &one); err != nil {
		t.Fatalf("One() in tx = %v", err)
	}
	if one.Name != "bob" {
		t.Errorf("One() in tx = %+v, want bob", one)
	}

	// A construction failure is intercepted before the transaction sees a
	// statement, so it does not block Commit.
	var none selectUser
	if err := tx.Select(users).Where(users.Col("missing").Eq(1)).One(ctx, &none); !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("invalid One() in tx = %v, want ErrInvalidQuery", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Errorf("Commit() after construction failure = %v, want nil", err)
	}
}

func TestSelectValuesAreBoundAsParameters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	// A value resembling SQL injection must be treated as data, not SQL.
	var found []selectUser
	malicious := "alice' OR '1'='1"
	if err := db.Select(users).Where(users.Col("name").Eq(malicious)).All(ctx, &found); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(found) != 0 {
		t.Errorf("All() with malicious literal returned %d rows, want 0", len(found))
	}
}
