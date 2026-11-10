package builders

import (
	"context"
	"database/sql"

	"github.com/mineot/krios/internal/dialects"
)

type Config struct {
	DB      *sql.DB
	Dialect dialects.Dialect
	Context context.Context
}
