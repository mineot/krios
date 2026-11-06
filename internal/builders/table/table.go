// Package table builds SQL references to a single table.
package table

import (
	"errors"
	"strings"

	"github.com/mineot/krios/internal/dialects"
)

// Table stores unquoted identifiers and retains the first construction error.
// Instances are mutable and must not be shared by concurrent builders.
type Table struct {
	// Name is a table name whose dot-separated components are quoted individually.
	Name string
	// Dialect selects the syntax used when rendering identifiers.
	Dialect dialects.Dialect
	alias   string
	schema  string
	quoter  dialects.Quoter
	err     error
}

// Err returns the first error. Create a new Table to start again after an error.
func (t *Table) Err() error {
	return t.err
}

// As replaces the alias unless this table has encountered an error.
func (t *Table) As(aliasName string) *Table {
	if t.validate(aliasName) {
		t.alias = aliasName
	}

	return t
}

// Schema replaces the qualifier supplied in Name unless an error occurred.
func (t *Table) Schema(schemaName string) *Table {
	if t.validate(schemaName) {
		t.schema = schemaName
	}

	return t
}

// validate checks a nonempty identifier and the current dialect without clearing errors.
func (t *Table) validate(name string) bool {
	if t.err != nil {
		return false
	}

	if name == "" {
		t.err = errors.New("krios: empty table identifier")
		return false
	}

	t.quote(name)

	return t.err == nil
}

// quote renders one component and transfers any quoting error to the table.
func (t *Table) quote(name string) string {
	if t.err != nil {
		return ""
	}

	t.quoter.Dialect = t.Dialect
	quoted := t.quoter.Quotes(name)
	t.err = t.quoter.Err()

	return quoted
}

// Build renders a table reference without FROM, or an empty string on error.
// Check Err after calling Build.
func (t *Table) Build() string {
	if t.err != nil {
		return ""
	}

	parts := strings.Split(t.Name, ".")

	for _, part := range parts {
		if !t.validate(part) {
			return ""
		}
	}

	// An explicit schema replaces any qualifier while preserving the table name.
	if t.schema != "" {
		parts = []string{t.schema, parts[len(parts)-1]}
	}

	for i, part := range parts {
		parts[i] = t.quote(part)
	}

	sql := strings.Join(parts, ".")

	if t.alias != "" {
		sql += " AS " + t.quote(t.alias)
	}

	if t.err != nil {
		return ""
	}

	return sql
}
