package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/sqlfile"
)

// createHistoryDDL is the infrastructure DDL that creates the migrations
// history table (E08-P01 §7). It is dialect-specific and idempotent. It is not
// a consumer migration: it is emitted once, before the first step, when the
// schema has no history.
func createHistoryDDL(d dialect.Dialect) string {
	table := d.QuoteIdent(historyTable)

	switch d.Name() {
	case "mysql":
		return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
version INT NOT NULL,
filename VARCHAR(255) NOT NULL,
direction VARCHAR(4) NOT NULL,
status VARCHAR(12) NOT NULL,
checksum VARCHAR(64) NULL,
applied_at TIMESTAMP NULL,
error TEXT NULL,
UNIQUE KEY %s (version, filename, direction)
)`, table, d.QuoteIdent(historyTable+"_unique"))
	default:
		return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
version INT NOT NULL,
filename VARCHAR(255) NOT NULL,
direction VARCHAR(4) NOT NULL,
status VARCHAR(12) NOT NULL,
checksum VARCHAR(64),
applied_at TIMESTAMP,
error TEXT,
UNIQUE (version, filename, direction)
)`, table)
	}
}

// checksum returns the lowercase SHA-256 hex digest of a file's text, stored
// in the history when a file is recorded (E08-P01 §7).
func checksum(f *sqlfile.SQLFile) string {
	sum := sha256.Sum256([]byte(f.Text()))

	return hex.EncodeToString(sum[:])
}

// insertInProgress records a step as in_progress with its checksum before the
// file runs (E08-P01 §7.1). filename is the exact resource name.
func insertInProgress(ctx context.Context, ex statement, d dialect.Dialect, step Step) error {
	query := fmt.Sprintf(
		"INSERT INTO %s (version, filename, direction, status, checksum) VALUES (%s, %s, %s, %s, %s)",
		d.QuoteIdent(historyTable), placeholder(d, 1), placeholder(d, 2), placeholder(d, 3), placeholder(d, 4), placeholder(d, 5),
	)

	_, err := ex.ExecContext(ctx, query, step.Version, step.File.Name(), step.Direction, StatusInProgress, checksum(step.File))

	return err
}

// markApplied transitions a step to applied and stamps applied_at. A failed
// record is never overwritten this way by the library (E08-P01 §7).
func markApplied(ctx context.Context, ex statement, d dialect.Dialect, step Step) error {
	query := fmt.Sprintf(
		"UPDATE %s SET status = %s, applied_at = %s WHERE version = %s AND filename = %s AND direction = %s",
		d.QuoteIdent(historyTable), placeholder(d, 1), placeholder(d, 2), placeholder(d, 3), placeholder(d, 4), placeholder(d, 5),
	)

	_, err := ex.ExecContext(ctx, query, StatusApplied, time.Now().UTC(), step.Version, step.File.Name(), step.Direction)

	return err
}

// markFailed transitions a step to failed and stores a summarized error. It
// never marks the step applied (E08-P01 §7.1).
func markFailed(ctx context.Context, ex statement, d dialect.Dialect, step Step, cause error) error {
	query := fmt.Sprintf(
		"UPDATE %s SET status = %s, error = %s WHERE version = %s AND filename = %s AND direction = %s",
		d.QuoteIdent(historyTable), placeholder(d, 1), placeholder(d, 2), placeholder(d, 3), placeholder(d, 4), placeholder(d, 5),
	)

	_, err := ex.ExecContext(ctx, query, StatusFailed, summarize(cause), step.Version, step.File.Name(), step.Direction)

	return err
}

// summarize bounds an error message for the error column. A nil cause yields
// an empty summary.
func summarize(err error) string {
	if err == nil {
		return ""
	}

	const limit = 1000

	msg := err.Error()

	if len(msg) > limit {
		return msg[:limit]
	}

	return msg
}

// placeholder renders the n-th parameter marker for the dialect: positional
// $n for PostgreSQL and a bare ? for SQLite and MySQL, matching what the
// configured driver accepts. Migrations never translate markers in consumer
// files; this only builds the library's own history statements.
func placeholder(d dialect.Dialect, n int) string {
	if d.Name() == "postgres" {
		return fmt.Sprintf("$%d", n)
	}

	return "?"
}
