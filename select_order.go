package krios

// OrderBy appends terms in order. An empty call leaves the query unchanged.
func (q SelectBuilder) OrderBy(terms ...OrderTerm) SelectBuilder {
	q.query = q.query.OrderBy(terms...)
	return q
}
