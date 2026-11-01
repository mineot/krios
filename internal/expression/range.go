package expression

// Between constructs an inclusive range with exactly two parameter bounds.
func Between(column string, lower, upper any) Predicate {
	return interval(betweenPredicate, column, lower, upper)
}

// NotBetween constructs a negated range with exactly two parameter bounds.
func NotBetween(column string, lower, upper any) Predicate {
	return interval(notBetweenPredicate, column, lower, upper)
}

func interval(kind predicateKind, column string, lower, upper any) Predicate {
	return Predicate{node: &predicateNode{
		kind: kind, operands: []Expr{Column(column), Value(lower), Value(upper)},
	}}
}
