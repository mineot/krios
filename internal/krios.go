package krios

import (
	"github.com/mineot/krios/internal/dialects"
)

type (
	Dialect = dialects.Dialect
)

const (
	MysqlDialect    = dialects.Mysql
	PostgresDialect = dialects.Postgres
	SQLiteDialect   = dialects.SQLite
)
