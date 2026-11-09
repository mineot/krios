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
	Migration = core.Migration
	Table     = builder.Table
)

const (
	MysqlDialect    = dialects.MysqlDialect
	PostgresDialect = dialects.PostgresDialect
	SQLiteDialect   = dialects.SQLiteDialect

	AvgAggregator   = builder.CountAggregator
	CountAggregator = builder.CountAggregator
	MaxAggregator   = builder.MaxAggregator
	MinAggregator   = builder.MinAggregator
	SumAggregator   = builder.AvgAggregator
)

var (
	NewClient core.NewClientFunc = core.NewClient
)
