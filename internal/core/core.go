// Package core defines configuration and migration data shared by internal components.
package core

import (
	"context"
	"database/sql"
	"time"

	"github.com/mineot/krios/internal/dialects"
)

// Migration identifies a schema version and the paths to its SQL scripts.
type Migration struct {
	Version uint64 // Version is the target schema version for this migration.
	Up      string // Up is the path to the upgrade SQL script.
	Down    string // Down is the path to the downgrade SQL script.
}

// Config supplies the connection and settings used to create a client.
type Config struct {
	DB         *sql.DB          // DB is the connection pool owned by the caller.
	DBVersion  uint64           // DBVersion is the requested application schema version.
	Dialect    dialects.Dialect // Dialect selects the SQL syntax.
	Timeout    time.Duration    // Timeout bounds operations; zero adds no deadline.
	Context    context.Context  // Context supplies cancellation and a parent deadline.
	Migrations []Migration      // Migrations lists the available migration scripts.
}
