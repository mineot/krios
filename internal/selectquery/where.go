package selectquery

import (
	"slices"

	"github.com/mineot/krios/internal/expression"
)

// Where appends an explicit AND group, including invalid empty groups.
func (q Builder) Where(predicates ...expression.Predicate) Builder {
	q.where = append(slices.Clone(q.where), expression.Where(predicates...))
	return q
}

func (q Builder) condition() expression.Predicate {
	if len(q.where) == 1 {
		return q.where[0]
	}
	return expression.And(q.where...)
}
