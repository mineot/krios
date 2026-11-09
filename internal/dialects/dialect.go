package dialects

type Dialect string

const (
	SQLiteDialect   Dialect = "sqlite"
	PostgresDialect Dialect = "postgres"
	MysqlDialect    Dialect = "mysql"
)
