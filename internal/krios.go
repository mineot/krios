package krios

import (
	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type (
	Dialect = dialects.Dialect
)

const (
	SQLite   = dialects.SQLite
	Postgres = dialects.Postgres
	Mysql    = dialects.Mysql
)

var (
	Config    core.Config
	Migration core.Migration
)
