package krios

import "github.com/mineot/krios/internal/expression"

// IsNull constructs an explicit NULL predicate.
func IsNull(column string) Predicate {
	return expression.IsNull(column)
}

// IsNotNull constructs an explicit NULL predicate.
func IsNotNull(column string) Predicate {
	return expression.IsNotNull(column)
}
