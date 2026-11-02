package krios

// Where appends a group combined with previous calls by AND.
// An explicit empty group remains invalid until Build or Exec reports it.
func (q SelectBuilder) Where(predicates ...Predicate) SelectBuilder {
	q.query = q.query.Where(predicates...)
	return q
}
