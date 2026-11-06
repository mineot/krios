package dialects

type Dialect string

const (
	ErrInvalidDialect = "Krios: Invalid Dialect: %q"

	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
	Mysql    Dialect = "mysql"
)
