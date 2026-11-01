package expression

// Count constructs an aggregate expression without evaluating it.
func Count(column string) Expr {
	return Expr{kind: aggregateExpr, aggregate: countAggregate, name: column}
}

// Avg constructs an aggregate expression without evaluating it.
func Avg(column string) Expr {
	return Expr{kind: aggregateExpr, aggregate: avgAggregate, name: column}
}

// Sum constructs an aggregate expression without evaluating it.
func Sum(column string) Expr {
	return Expr{kind: aggregateExpr, aggregate: sumAggregate, name: column}
}

// Min constructs an aggregate expression without evaluating it.
func Min(column string) Expr {
	return Expr{kind: aggregateExpr, aggregate: minAggregate, name: column}
}

// Max constructs an aggregate expression without evaluating it.
func Max(column string) Expr {
	return Expr{kind: aggregateExpr, aggregate: maxAggregate, name: column}
}
