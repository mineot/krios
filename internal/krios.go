package krios

import (
	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type (
	Client    = core.Client
	Config    = core.Config
	Dialect   = dialects.Dialect
	Migration = core.Migration
)

const (
	MysqlDialect    = dialects.MysqlDialect
	PostgresDialect = dialects.PostgresDialect
	SQLiteDialect   = dialects.SQLiteDialect
)

var (
	NewClient core.NewClientFunc = core.NewClient
)
