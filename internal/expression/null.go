package expression

// IsNull constructs an explicit SQL NULL test.
func IsNull(column string) Predicate {
	return Predicate{node: &predicateNode{kind: isNullPredicate, operands: []Expr{Column(column)}}}
}

// IsNotNull constructs an explicit SQL non-NULL test.
func IsNotNull(column string) Predicate {
	return Predicate{node: &predicateNode{kind: isNotNullPredicate, operands: []Expr{Column(column)}}}
}
