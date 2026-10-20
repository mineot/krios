package dialect

// ReturningDialect reports whether a dialect supports the RETURNING clause of
// write statements (E06-P01 §7). Every concrete dialect type implements it in
// addition to Dialect and MigrationDialect; write builders consume it through
// this interface and reject a write request before execution when the
// capability is absent.
type ReturningDialect interface {
	// SupportsReturning reports whether INSERT, UPDATE and DELETE accept a
	// RETURNING clause.
	SupportsReturning() bool
}

// SQLite has accepted RETURNING since 3.35; PostgreSQL since 9.5. MySQL has
// no RETURNING clause.
func (sqlite) SupportsReturning() bool   { return true }
func (mysql) SupportsReturning() bool    { return false }
func (postgres) SupportsReturning() bool { return true }
