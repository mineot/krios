// Package selects holds the internal state for SELECT query construction.
package selects

import (
	"errors"

	"github.com/mineot/krios/internal/builders/table"
	"github.com/mineot/krios/internal/dialects"
)

// Select stores query state and retains the first construction error.
// Instances are mutable and must not be used concurrently.
type Select struct {
	columns   []string
	from      *table.Table
	joins     []string
	criterias []string
	orders    []string
	groups    []string
	paginate  []string
	dialect   dialects.Dialect
	err       error
}

// NewSelect creates a query builder using the supplied dialect.
func NewSelect(dialect dialects.Dialect) *Select {
	s := &Select{dialect: dialect}

	switch dialect {
	case dialects.SQLite, dialects.Postgres, dialects.Mysql:
	default:
		s.err = errors.New("krios: invalid dialect")
	}

	return s
}

// Err returns the first error. Create a new Select to start again after an error.
func (s *Select) Err() error {
	return s.err
}

// From replaces the main table and clears pagination for the previous source.
func (s *Select) From(tableName string) *Select {
	if s.err != nil {
		return s
	}

	source := &table.Table{Name: tableName, Dialect: s.dialect}

	// Validate the source before replacing the existing table.
	source.Build()

	if source.Err() != nil {
		s.err = source.Err()
		return s
	}

	s.from = source
	s.paginate = nil

	return s
}

// As sets the main table alias after From has selected a source.
func (s *Select) As(aliasName string) *Select {
	if !s.hasSource() {
		return s
	}

	s.from.As(aliasName)
	s.err = s.from.Err()

	return s
}

// Schema sets the main table qualifier after From has selected a source.
func (s *Select) Schema(schemaName string) *Select {
	if !s.hasSource() {
		return s
	}

	s.from.Schema(schemaName)
	s.err = s.from.Err()

	return s
}

// hasSource records a missing source without replacing an earlier error.
func (s *Select) hasSource() bool {
	if s.err != nil {
		return false
	}

	if s.from == nil {
		s.err = errors.New("krios: From must select a table first")
		return false
	}

	return true
}

// Build currently renders only the main table reference, not a complete SELECT.
// It returns an empty string on failure; check Err after calling Build.
func (s *Select) Build() string {
	if !s.hasSource() {
		return ""
	}

	sql := s.from.Build()
	s.err = s.from.Err()

	if s.err != nil {
		return ""
	}

	return sql
}
