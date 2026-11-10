package krios

import (
	"github.com/mineot/krios/internal/builders"
	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type (
	Client    = core.Client
	Column    = core.Column
	Config    = core.Config
	Dialect   = dialects.Dialect
	Migration = core.Migration
	Select    = core.Select
	Table     = core.Table
)

const (
	Mysql    = dialects.Mysql
	Postgres = dialects.Postgres
	SQLite   = dialects.SQLite

	Avg   = builders.Avg
	Count = builders.Count
	Max   = builders.Max
	Min   = builders.Min
	Sum   = builders.Sum
)

var (
	NewClient core.NewClientFunc = core.NewClient
)
