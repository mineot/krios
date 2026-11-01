package krios

import "github.com/mineot/krios/internal/expression"

// Count constructs an aggregate expression; As assigns a projection alias.
func Count(column string) Expr {
	return expression.Count(column)
}

// Avg constructs an aggregate expression; As assigns a projection alias.
func Avg(column string) Expr {
	return expression.Avg(column)
}

// Sum constructs an aggregate expression; As assigns a projection alias.
func Sum(column string) Expr {
	return expression.Sum(column)
}

// Min constructs an aggregate expression; As assigns a projection alias.
func Min(column string) Expr {
	return expression.Min(column)
}

// Max constructs an aggregate expression; As assigns a projection alias.
func Max(column string) Expr {
	return expression.Max(column)
}
