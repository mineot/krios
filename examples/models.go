package examples

import (
	"fmt"
	"strings"
)

// Status is the account lifecycle state stored in users.status. It implements
// the field-level krios.Validator and krios.Formatter interfaces detected by
// the entity descriptor.
type Status string

// Supported account states.
const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

// Validate implements krios.Validator and rejects unknown states.
func (s Status) Validate() error {
	switch s {
	case StatusActive, StatusInactive:
		return nil
	default:
		return fmt.Errorf("unsupported status %q", s)
	}
}

// Format implements krios.Formatter and returns the presentation text used by
// Entity.Format. Database operations always use the original value.
func (s Status) Format() (string, error) {
	return strings.ToUpper(string(s)), nil
}

// User maps the users table. Tags name the columns; Bio is a pointer so a
// missing value maps to SQL NULL.
type User struct {
	ID     int64   `krios:"id,pk"`
	Name   string  `krios:"name"`
	Email  string  `krios:"email"`
	Bio    *string `krios:"bio"`
	Status Status  `krios:"status"`
}

// TableName overrides the snake_case derivation of the type name.
func (User) TableName() string { return "users" }

// Task maps the tasks table. OwnerID and Done carry no tag: their columns are
// derived from the field names as snake_case (owner_id, done).
type Task struct {
	ID      int64  `krios:"id,pk"`
	Title   string `krios:"title"`
	OwnerID int64
	Done    bool
}

// TableName overrides the snake_case derivation of the type name.
func (Task) TableName() string { return "tasks" }
