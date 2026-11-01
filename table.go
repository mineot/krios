package krios

import "github.com/mineot/krios/internal/expression"

// Table constructs an unqualified table reference; use InSchema for qualification.
func Table(name string) TableRef {
	return expression.Table(name)
}
