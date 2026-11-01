package expression

// Asc constructs ascending ordering for a column without a wildcard.
func Asc(column string) OrderTerm {
	return OrderTerm{column: column, direction: ascending}
}

// Desc constructs descending ordering for a column without a wildcard.
func Desc(column string) OrderTerm {
	return OrderTerm{column: column, direction: descending}
}

// NullsFirst returns a term with explicit NULL-first ordering.
func (order OrderTerm) NullsFirst() OrderTerm {
	order.nulls = nullsFirst
	return order
}

// NullsLast returns a term with explicit NULL-last ordering.
func (order OrderTerm) NullsLast() OrderTerm {
	order.nulls = nullsLast
	return order
}
