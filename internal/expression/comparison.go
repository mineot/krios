package expression

// Eq constructs equality between a column and a parameter.
func Eq(column string, value any) Predicate {
	return EqExpr(Column(column), Value(value))
}

// EqExpr constructs equality between two expressions.
func EqExpr(left, right Expr) Predicate {
	return comparison(eqPredicate, left, right)
}

// Ne constructs inequality between a column and a parameter.
func Ne(column string, value any) Predicate {
	return NeExpr(Column(column), Value(value))
}

// NeExpr constructs inequality between two expressions.
func NeExpr(left, right Expr) Predicate {
	return comparison(nePredicate, left, right)
}

// Lt constructs less-than comparison between a column and a parameter.
func Lt(column string, value any) Predicate {
	return LtExpr(Column(column), Value(value))
}

// LtExpr constructs less-than comparison between two expressions.
func LtExpr(left, right Expr) Predicate {
	return comparison(ltPredicate, left, right)
}

// Lte constructs less-than-or-equal comparison between a column and a parameter.
func Lte(column string, value any) Predicate {
	return LteExpr(Column(column), Value(value))
}

// LteExpr constructs less-than-or-equal comparison between two expressions.
func LteExpr(left, right Expr) Predicate {
	return comparison(ltePredicate, left, right)
}

// Gt constructs greater-than comparison between a column and a parameter.
func Gt(column string, value any) Predicate {
	return GtExpr(Column(column), Value(value))
}

// GtExpr constructs greater-than comparison between two expressions.
func GtExpr(left, right Expr) Predicate {
	return comparison(gtPredicate, left, right)
}

// Gte constructs greater-than-or-equal comparison between a column and a parameter.
func Gte(column string, value any) Predicate {
	return GteExpr(Column(column), Value(value))
}

// GteExpr constructs greater-than-or-equal comparison between two expressions.
func GteExpr(left, right Expr) Predicate {
	return comparison(gtePredicate, left, right)
}

func comparison(kind predicateKind, left, right Expr) Predicate {
	return Predicate{node: &predicateNode{kind: kind, operands: []Expr{left, right}}}
}
