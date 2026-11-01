package krios

import "github.com/mineot/krios/internal/expression"

// Value constructs a parameter, including NULL, without evaluation or deep copying.
func Value(value any) Expr {
	return expression.Value(value)
}
