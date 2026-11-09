package krios

import (
	"github.com/mineot/krios/internal/builder"
	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type (
	Client    = core.Client
	Column    = builder.Column
	Config    = core.Config
	Dialect   = dialects.Dialect
	GroupBy   = builder.GroupBy
	Migration = core.Migration
	OrderBy   = builder.OrderBy
	Table     = builder.Table
)

const (
	Mysql    = dialects.Mysql
	Postgres = dialects.Postgres
	SQLite   = dialects.SQLite

	Avg   = builder.Avg
	Count = builder.Count
	Max   = builder.Max
	Min   = builder.Min
	Sum   = builder.Sum

	Asc  = builder.Asc
	Desc = builder.Desc
)

var (
	NewClient core.NewClientFunc = core.NewClient
)
