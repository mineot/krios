package krios

// Distinct enables DISTINCT; repeated calls are idempotent.
func (q SelectBuilder) Distinct() SelectBuilder {
	q.query = q.query.Distinct()
	return q
}

// All replaces the projection with *, preserving DISTINCT and all other clauses.
func (q SelectBuilder) All() SelectBuilder {
	q.query = q.query.All()
	return q
}

// Columns replaces the projection. An explicit empty projection is invalid.
func (q SelectBuilder) Columns(columns ...Expr) SelectBuilder {
	q.query = q.query.Columns(columns...)
	return q
}
