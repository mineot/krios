package core

import (
	"context"
	"database/sql"
)

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var (
	_ executor = (*sql.DB)(nil)
	_ executor = (*sql.Tx)(nil)
)
