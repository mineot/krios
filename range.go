package krios

import "github.com/mineot/krios/internal/expression"

// Between constructs a range predicate with exactly two parameter bounds.
func Between(column string, lower, upper any) Predicate {
	return expression.Between(column, lower, upper)
}

// NotBetween constructs a range predicate with exactly two parameter bounds.
func NotBetween(column string, lower, upper any) Predicate {
	return expression.NotBetween(column, lower, upper)
}
