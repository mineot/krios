package dialects

type Dialect string

const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
	Mysql    Dialect = "mysql"
)
