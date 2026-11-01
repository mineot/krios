package krios

import "github.com/mineot/krios/internal/expression"

// Eq constructs equality between a column and a parameter.
func Eq(column string, value any) Predicate {
	return expression.Eq(column, value)
}

// EqExpr constructs equality between two expressions.
func EqExpr(left, right Expr) Predicate {
	return expression.EqExpr(left, right)
}

// Ne constructs inequality between a column and a parameter.
func Ne(column string, value any) Predicate {
	return expression.Ne(column, value)
}

// NeExpr constructs inequality between two expressions.
func NeExpr(left, right Expr) Predicate {
	return expression.NeExpr(left, right)
}

// Lt constructs less-than comparison between a column and a parameter.
func Lt(column string, value any) Predicate {
	return expression.Lt(column, value)
}

// LtExpr constructs less-than comparison between two expressions.
func LtExpr(left, right Expr) Predicate {
	return expression.LtExpr(left, right)
}

// Lte constructs less-than-or-equal comparison between a column and a parameter.
func Lte(column string, value any) Predicate {
	return expression.Lte(column, value)
}

// LteExpr constructs less-than-or-equal comparison between two expressions.
func LteExpr(left, right Expr) Predicate {
	return expression.LteExpr(left, right)
}

// Gt constructs greater-than comparison between a column and a parameter.
func Gt(column string, value any) Predicate {
	return expression.Gt(column, value)
}

// GtExpr constructs greater-than comparison between two expressions.
func GtExpr(left, right Expr) Predicate {
	return expression.GtExpr(left, right)
}

// Gte constructs greater-than-or-equal comparison between a column and a parameter.
func Gte(column string, value any) Predicate {
	return expression.Gte(column, value)
}

// GteExpr constructs greater-than-or-equal comparison between two expressions.
func GteExpr(left, right Expr) Predicate {
	return expression.GteExpr(left, right)
}
