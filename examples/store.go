package examples

import (
	"context"
	"database/sql"
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

// Store owns the database access for KriosTasks. A Store is safe for
// concurrent use and typically serves the whole application.
type Store struct {
	db *krios.DB
}

// Close releases the database access. It is idempotent.
func (s *Store) Close() error { return s.db.Close() }

// CreateSchema creates both tables with raw SQL. Migrations (E08) are not
// implemented yet, so the consumer owns the DDL; Krios executes it without
// translation between dialects.
func (s *Store) CreateSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			bio TEXT,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			owner_id INTEGER NOT NULL,
			done BOOLEAN NOT NULL DEFAULT FALSE
		)`,
	}
	for _, ddl := range statements {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	return nil
}

// CreateUser validates user, inserts it and stores the generated id. The
// placeholders follow the SQLite/MySQL style ("?"); PostgreSQL consumer SQL
// uses "$1", "$2", ... because Krios never rewrites it.
func (s *Store) CreateUser(ctx context.Context, user *User) error {
	if err := userEntity.Validate(user); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		user.Name, user.Email, user.Bio, user.Status)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		user.ID = id
	}
	return nil
}

// CreateTask inserts one task owned by task.OwnerID.
func (s *Store) CreateTask(ctx context.Context, task *Task) error {
	if err := taskEntity.Validate(task); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO tasks (title, owner_id, done) VALUES (?, ?, ?)",
		task.Title, task.OwnerID, task.Done)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		task.ID = id
	}
	return nil
}

// CreateUserWithFirstTasks registers a user and seeds their first tasks inside
// one transaction. Any failure rolls the whole group back through the deferred
// Rollback, so the user is never persisted without its tasks.
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

	res, err := tx.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		user.Name, user.Email, user.Bio, user.Status)
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
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO tasks (title, owner_id, done) VALUES (?, ?, ?)",
			title, user.ID, false); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// TransferTasks reassigns every open task of one owner to another inside a
// transaction and reports how many rows moved.
func (s *Store) TransferTasks(ctx context.Context, from, to int64) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM users WHERE id = ?", to).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("target user %d does not exist", to)
	}
	if err != nil {
		return 0, err
	}

	res, err := tx.ExecContext(ctx,
		"UPDATE tasks SET owner_id = ? WHERE owner_id = ? AND done = ?",
		to, from, false)
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

// ListUsers reads every user ordered by id and maps the result set by column
// name. Rows.All always closes the iterator and replaces the slice only on
// success.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, email, bio, status FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	var users []User
	if err := rows.All(&users); err != nil {
		return nil, err
	}
	return users, nil
}

// UserByID reads one user. Row.Scan maps positionally, so the select list must
// follow the declaration order of the struct fields; sql.ErrNoRows is
// preserved for existence checks.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, email, bio, status FROM users WHERE id = ?", id,
	).Scan(&user)
	return user, err
}

// UserByEmail reads one user by its unique email with the same positional
// mapping and the same sql.ErrNoRows contract.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, email, bio, status FROM users WHERE email = ?", email,
	).Scan(&user)
	return user, err
}

// CountOpenTasks returns the number of open tasks of an owner.
func (s *Store) CountOpenTasks(ctx context.Context, ownerID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tasks WHERE owner_id = ? AND done = ?",
		ownerID, false,
	).Scan(&count)
	return count, err
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
