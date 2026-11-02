package selectquery

import (
	"slices"

	"github.com/mineot/krios/internal/expression"
)

// Distinct enables DISTINCT without changing the projection.
func (q Builder) Distinct() Builder {
	q.distinct = true
	return q
}

// All replaces the projection with *, preserving DISTINCT and other clauses.
func (q Builder) All() Builder {
	q.columnsSet = false
	q.columns = nil
	return q
}

// Columns replaces the projection. An explicit empty list is invalid at Build.
func (q Builder) Columns(columns ...expression.Expr) Builder {
	q.columnsSet = true
	q.columns = slices.Clone(columns)
	return q
}
