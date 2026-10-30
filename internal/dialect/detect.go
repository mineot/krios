package dialect

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mineot/krios/internal/errs"
)

// Detect reads the server version and derives known capabilities.
func Detect(ctx context.Context, pool *sql.DB, kind Dialect) (Info, error) {
	if ctx == nil || pool == nil || !Valid(kind) {
		return Info{}, fmt.Errorf("%w: invalid version detection arguments", errs.ErrInvalidConfig)
	}

	var query string

	switch kind {
	case Postgres:
		query = "SELECT current_setting('server_version_num')"
	case MySQL:
		query = "SELECT VERSION()"
	case SQLite:
		query = "SELECT sqlite_version()"
	}

	var raw string
	if err := pool.QueryRowContext(ctx, query).Scan(&raw); err != nil {
		return Info{}, fmt.Errorf("%w: %w", errs.ErrVersionDetection, err)
	}

	if err := ctx.Err(); err != nil {
		return Info{}, fmt.Errorf("%w: %w", errs.ErrVersionDetection, err)
	}

	version, err := parseVersion(kind, raw)

	if err != nil {
		return Info{}, fmt.Errorf("%w: %w", errs.ErrVersionDetection, err)
	}

	if err := validateVersion(kind, version); err != nil {
		return Info{}, err
	}

	return Info{Dialect: kind, Version: version, Capabilities: capabilities(kind, version)}, nil
}
