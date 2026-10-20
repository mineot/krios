package examples

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/mineot/krios"
)

// Entity descriptors derived once per model type by krios.EntityOf.
var (
	userEntity = mustEntity[User]()
	taskEntity = mustEntity[Task]()
)

func mustEntity[T any]() *krios.Entity {
	ent, err := krios.EntityOf[T]()
	if err != nil {
		// An invalid model is a programming error caught at development time.
		panic(err)
	}
	return ent
}

// schemaFS embeds the DDL files into the executable, so the consumer does not
// ship or read them at run time. Each file holds a single statement, since
// Krios does not split multiple statements.
//
//go:embed schema/*.sql
var schemaFS embed.FS

// Schema resources loaded once at startup. Eager loading surfaces a missing or
// unreadable resource immediately, not on first use.
var (
	usersSchema = mustSQLFile(schemaFS, "schema/users.sql")
	tasksSchema = mustSQLFile(schemaFS, "schema/tasks.sql")
)

func mustSQLFile(fsys embed.FS, path string) *krios.SQLFile {
	f, err := krios.NewSQLFile(fsys, path)
	if err != nil {
		// A missing embedded resource is a build/packaging error.
		panic(err)
	}
	return f
}

// Store owns the database access for KriosTasks. A Store is safe for
// concurrent use and typically serves the whole application.
type Store struct {
	db *krios.DB
}

// Close releases the database access. It is idempotent.
func (s *Store) Close() error { return s.db.Close() }

// CreateSchema creates both tables from the embedded DDL files. This example
// keeps owning its DDL directly instead of using versioned migrations
// (DB.RunMigrations); Krios executes the file text without translation between
// dialects.
func (s *Store) CreateSchema(ctx context.Context) error {
	for _, schema := range []*krios.SQLFile{usersSchema, tasksSchema} {
		if _, err := s.db.File(schema).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// CreateUser validates user, inserts it with the entity write builder and
// stores the generated id.
func (s *Store) CreateUser(ctx context.Context, user *User) error {
	if err := userEntity.Validate(user); err != nil {
		return err
	}
	res, err := s.db.Insert(userEntity).SetValues(user).Omit("id").Exec(ctx)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		user.ID = id
	}
	return nil
}

// CreateTask inserts one task owned by task.OwnerID with the entity write
// builder.
func (s *Store) CreateTask(ctx context.Context, task *Task) error {
	if err := taskEntity.Validate(task); err != nil {
		return err
	}
	res, err := s.db.Insert(taskEntity).SetValues(task).Omit("id").Exec(ctx)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		task.ID = id
	}
	return nil
}

// CreateUserWithFirstTasks registers a user and seeds their first tasks inside
// one transaction using the entity write builders. Any failure rolls the whole
// group back through the deferred Rollback, so the user is never persisted
// without its tasks.
func (s *Store) CreateUserWithFirstTasks(ctx context.Context, user *User, titles ...string) error {
	if err := userEntity.Validate(user); err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	// Covers every early return; Rollback after a successful Commit is a no-op.
	defer tx.Rollback(ctx)

	res, err := tx.Insert(userEntity).SetValues(user).Omit("id").Exec(ctx)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		user.ID = id
	}
	for _, title := range titles {
		if title == "" {
			return errors.New("task title must not be empty")
		}
		task := &Task{Title: title, OwnerID: user.ID, Done: false}
		if err := taskEntity.Validate(task); err != nil {
			return err
		}
		if _, err := tx.Insert(taskEntity).SetValues(task).Omit("id").Exec(ctx); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// TransferTasks reassigns every open task of one owner to another inside a
// transaction using the UPDATE builder and reports how many rows moved.
func (s *Store) TransferTasks(ctx context.Context, from, to int64) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var target User
	err = tx.Select(userEntity).Where(userEntity.Col("id").Eq(to)).One(ctx, &target)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("target user %d does not exist", to)
	}
	if err != nil {
		return 0, err
	}

	res, err := tx.Update(taskEntity).
		SetValues(&Task{OwnerID: to}).
		Omit("id", "title", "done").
		Where(taskEntity.Col("owner_id").Eq(from), taskEntity.Col("done").Eq(false)).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	moved, _ := res.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return moved, nil
}

// RegisterAndCloneEmail demonstrates the blocked commit: after a successful
// insert, a second insert of the same email fails on the UNIQUE constraint.
// The transaction remembers that first error, so Commit rolls back
// automatically and returns an error matching it with errors.Is. The first
// insert is discarded with the rest of the group.
func (s *Store) RegisterAndCloneEmail(ctx context.Context, first, clone *User) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		first.Name, first.Email, first.Bio, first.Status); err != nil {
		return err
	}

	_, statementErr := tx.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		clone.Name, clone.Email, clone.Bio, clone.Status)
	if statementErr == nil {
		return errors.New("expected a duplicate email error")
	}

	// Explicit rollback remains the recommended response; this Commit shows the
	// extra protection when the consumer attempts it anyway.
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return errors.New("commit was not blocked by the statement error")
}

// ListUsers reads every user ordered by id. The E05 builder maps the result set
// by column name; Rows.All replaces the slice only on success.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	err := s.db.Select(userEntity).
		OrderBy(userEntity.Col("id").Asc()).
		All(ctx, &users)
	return users, err
}

// UserByID reads one user through the builder. One maps positionally, so the
// default selection follows the declaration order of the struct fields;
// sql.ErrNoRows is preserved for existence checks.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.Select(userEntity).
		Where(userEntity.Col("id").Eq(id)).
		One(ctx, &user)
	return user, err
}

// UserByEmail reads one user by its unique email with the same positional
// mapping and the same sql.ErrNoRows contract.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.db.Select(userEntity).
		Where(userEntity.Col("email").Eq(email)).
		One(ctx, &user)
	return user, err
}

// CountOpenTasks returns the number of open tasks of an owner. Count uses a
// derived-table query, so it remains exact even if the builder is extended with
// DISTINCT or grouping later.
func (s *Store) CountOpenTasks(ctx context.Context, ownerID int64) (int64, error) {
	return s.db.Select(taskEntity).
		Where(
			taskEntity.Col("owner_id").Eq(ownerID),
			taskEntity.Col("done").Eq(false),
		).
		Count(ctx)
}

// ListOpenTasksByOwnerEmail finds open tasks through an INNER JOIN. The base
// entity is tasks; users participates only in the join and filter.
func (s *Store) ListOpenTasksByOwnerEmail(ctx context.Context, email string) ([]Task, error) {
	var tasks []Task
	err := s.db.Select(taskEntity).
		Join(userEntity, krios.InnerJoin, userEntity.Col("id").Eq(taskEntity.Col("owner_id"))).
		Where(
			userEntity.Col("email").Eq(email),
			taskEntity.Col("done").Eq(false),
		).
		OrderBy(taskEntity.Col("id").Asc()).
		All(ctx, &tasks)
	return tasks, err
}

// UserTaskCount is a read model mapped by output alias. It demonstrates a
// grouped projection built with Fields.
type UserTaskCount struct {
	ID    int64  `krios:"id"`
	Name  string `krios:"name"`
	Tasks int64  `krios:"tasks"`
}

// ListUserTaskCounts returns every user with their task count. The LEFT JOIN
// keeps users without tasks, whose count is zero.
func (s *Store) ListUserTaskCounts(ctx context.Context) ([]UserTaskCount, error) {
	var counts []UserTaskCount
	err := s.db.Select(userEntity).
		Join(taskEntity, krios.LeftJoin, taskEntity.Col("owner_id").Eq(userEntity.Col("id"))).
		Fields(
			userEntity.Col("id"),
			userEntity.Col("name"),
			taskEntity.Col("id").Count().As("tasks"),
		).
		GroupBy(userEntity.Col("id"), userEntity.Col("name")).
		OrderBy(userEntity.Col("id").Asc()).
		All(ctx, &counts)
	return counts, err
}

// ListUsersPage returns one page of users together with the totals and
// navigation statistics computed by Paginate. The count and the page are
// independent queries on *DB.
func (s *Store) ListUsersPage(ctx context.Context, limit, offset int) ([]User, krios.Paginate, error) {
	var users []User
	page, err := s.db.Select(userEntity).
		OrderBy(userEntity.Col("id").Asc()).
		Paginate(ctx, limit, offset, &users)
	if err != nil {
		return nil, krios.Paginate{}, err
	}
	return users, page, nil
}

// ValidateUsers reports whether a batch of users satisfies every field-level
// Validator, stopping at the first invalid record with krios.ErrValidation.
func (s *Store) ValidateUsers(users []User) error {
	return userEntity.ValidateAll(users)
}

// PresentUser returns the display text of every mapped column of user. The
// formatted values are presentation-only; writes always use the original
// fields.
func (s *Store) PresentUser(user User) (map[string]string, error) {
	return userEntity.Format(user)
}
