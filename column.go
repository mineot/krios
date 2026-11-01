package krios

import "github.com/mineot/krios/internal/expression"

// Column constructs an identifier, not a SQL fragment; As assigns a projection alias.
func Column(name string) Expr {
	return expression.Column(name)
}
