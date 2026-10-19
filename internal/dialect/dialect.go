// Package dialect adapts Krios-generated SQL to each supported database. A
// dialect abstracts only the syntax points that differ between engines; the
// SQL structure is produced by a shared renderer. Dialects are internal
// details selected by config.DBType and are never exposed to consumers.
package dialect

import (
	"strconv"
	"strings"

	"github.com/mineot/krios/internal/config"
)

// Dialect adapts the points of generated SQL that differ between the
// supported databases.
type Dialect interface {
	// Name returns the dialect name: "sqlite", "mysql" or "postgres".
	Name() string

	// Placeholder returns the marker for the n-th parameter (1-based).
	Placeholder(n int) string

	// QuoteIdent quotes an identifier for use as a table or column
	// reference. Qualified names such as "schema.table" are split on dots
	// and every part is quoted.
	QuoteIdent(name string) string
}

// For returns the dialect for the configured database type, or nil for
// unsupported types. Config validation rejects unsupported types before they
// reach the access layer.
func For(t config.DBType) Dialect {
	switch t {
	case config.DBTypeSQLite:
		return sqlite{}
	case config.DBTypeMySQL:
		return mysql{}
	case config.DBTypePostgres:
		return postgres{}
	default:
		return nil
	}
}

func quoteDouble(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, ".")
}

func quoteBacktick(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = "`" + part + "`"
	}
	return strings.Join(parts, ".")
}

type sqlite struct{}

func (sqlite) Name() string                  { return "sqlite" }
func (sqlite) Placeholder(int) string        { return "?" }
func (sqlite) QuoteIdent(name string) string { return quoteDouble(name) }

type mysql struct{}

func (mysql) Name() string                  { return "mysql" }
func (mysql) Placeholder(int) string        { return "?" }
func (mysql) QuoteIdent(name string) string { return quoteBacktick(name) }

type postgres struct{}

func (postgres) Name() string                  { return "postgres" }
func (postgres) Placeholder(n int) string      { return "$" + strconv.Itoa(n) }
func (postgres) QuoteIdent(name string) string { return quoteDouble(name) }
