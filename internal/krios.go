package krios

import (
	"github.com/mineot/krios/internal/builders"
	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type (
	Client    = core.Client
	Config    = core.Config
	Dialect   = dialects.Dialect
	Migration = core.Migration
	Table     = builders.Table
)

const (
	Mysql    = dialects.Mysql
	Postgres = dialects.Postgres
	SQLite   = dialects.SQLite
)

var (
	NewClient core.NewClientFunc = core.NewClient
)
