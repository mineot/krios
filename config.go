package krios

import (
	"github.com/mineot/krios/internal/connection"
	"github.com/mineot/krios/internal/dialect"
)

// Config defines the driver, dialect, DSN and initialization timeout.
type Config = connection.Config

// Dialect identifies the SQL dialect independently of the driver name.
type Dialect = dialect.Dialect

const (
	Postgres = dialect.Postgres
	MySQL    = dialect.MySQL
	SQLite   = dialect.SQLite
)
