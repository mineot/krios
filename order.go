package krios

import "github.com/mineot/krios/internal/expression"

// Asc constructs an ordering term; NullsFirst/NullsLast set NULL placement.
func Asc(column string) OrderTerm {
	return expression.Asc(column)
}

// Desc constructs an ordering term; NullsFirst/NullsLast set NULL placement.
func Desc(column string) OrderTerm {
	return expression.Desc(column)
}
