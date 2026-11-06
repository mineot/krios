// Package dialects defines supported SQL dialects and identifier quoting.
package dialects

// Dialect identifies the SQL syntax used when generating queries.
type Dialect string

const (
	// SQLite uses SQLite SQL syntax.
	SQLite Dialect = "sqlite"
	// Postgres uses PostgreSQL SQL syntax.
	Postgres Dialect = "postgres"
	// Mysql uses MySQL SQL syntax.
	Mysql Dialect = "mysql"
)
