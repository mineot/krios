package core

import (
	"context"
	"database/sql"
	"time"

	"github.com/mineot/krios/internal/dialects"
)

type Migration struct {
	Version uint64
	Up      string
	Down    string
}

type Config struct {
	DB         *sql.DB
	DBVersion  uint64
	Dialect    dialects.Dialect
	Timeout    time.Duration
	Context    context.Context
	Migrations []Migration
}
