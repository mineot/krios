// Package dialect identifies database dialects, versions and capabilities.
package dialect

// Dialect is independent of the driver registered with database/sql.
type Dialect string

const (
	Postgres Dialect = "pg"
	MySQL    Dialect = "mysql"
	SQLite   Dialect = "sqlite"
)

// Valid reports whether the dialect is supported by Krios.
func Valid(d Dialect) bool {
	return d == Postgres || d == MySQL || d == SQLite
}
