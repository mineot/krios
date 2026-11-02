package krios

// Build compiles SQL and arguments without I/O, including after the bound Tx
// finishes. On failure it returns no partial SQL or arguments.
func (q SelectBuilder) Build() (string, []any, error) {
	return q.query.Build()
}
