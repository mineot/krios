package krios

import (
	"github.com/mineot/krios/internal/clients"
	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type (
	Client    = clients.Client
	Config    = core.Config
	Dialect   = dialects.Dialect
	Migration = core.Migration
)

const (
	MysqlDialect    = dialects.Mysql
	PostgresDialect = dialects.Postgres
	SQLiteDialect   = dialects.SQLite
)

var (
	NewClient clients.NewClientFunc = clients.NewClient
)
