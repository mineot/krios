package krios

import "github.com/mineot/krios/internal/expression"

// In constructs membership; an empty list is always false.
func In(column string, values ...any) Predicate {
	return expression.In(column, values...)
}

// NotIn constructs non-membership; an empty list is always true.
func NotIn(column string, values ...any) Predicate {
	return expression.NotIn(column, values...)
}
