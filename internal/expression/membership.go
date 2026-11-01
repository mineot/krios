package expression

import "slices"

// In constructs membership; an empty list is always false.
func In(column string, values ...any) Predicate {
	return membership(inPredicate, column, values)
}

// NotIn constructs non-membership; an empty list is always true.
func NotIn(column string, values ...any) Predicate {
	return membership(notInPredicate, column, values)
}

func membership(kind predicateKind, column string, values []any) Predicate {
	return Predicate{node: &predicateNode{
		kind: kind, operands: []Expr{Column(column)}, values: slices.Clone(values),
	}}
}
