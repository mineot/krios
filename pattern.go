package krios

import "github.com/mineot/krios/internal/expression"

// Like constructs a comparison using a parameterized pattern.
func Like(column, pattern string) Predicate {
	return expression.Like(column, pattern)
}

// NotLike constructs a comparison using a parameterized pattern.
func NotLike(column, pattern string) Predicate {
	return expression.NotLike(column, pattern)
}
