package selectquery

import (
	"slices"

	"github.com/mineot/krios/internal/expression"
)

// OrderBy appends ordering terms; an empty call leaves the query unchanged.
func (q Builder) OrderBy(terms ...expression.OrderTerm) Builder {
	if len(terms) != 0 {
		q.order = append(slices.Clone(q.order), terms...)
	}
	return q
}
