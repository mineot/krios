// Package krios groups internal client and dialect entry points.
package krios

import (
	"github.com/mineot/krios/internal/clients"
	"github.com/mineot/krios/internal/core"
	"github.com/mineot/krios/internal/dialects"
)

type (
	// Dialect identifies a supported SQL syntax.
	Dialect = dialects.Dialect
)

const (
	// SQLite selects SQLite syntax.
	SQLite = dialects.SQLite
	// Postgres selects PostgreSQL syntax.
	Postgres = dialects.Postgres
	// Mysql selects MySQL syntax.
	Mysql = dialects.Mysql
)

var (
	// Config is a package-level configuration value.
	Config core.Config
	// Migration is a package-level migration value.
	Migration core.Migration
	// Client is a package-level client value.
	Client clients.Client
	// NewClient initializes a client from the supplied configuration.
	NewClient clients.NewCLientFunc = clients.NewClient
)
