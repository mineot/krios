package dialects

import (
	"errors"
	"strings"
)

// Quoter escapes identifiers and retains the first error.
// Create a new Quoter to start again after an error.
type Quoter struct {
	Dialect Dialect
	err     error
}

// Quotes renders one identifier component, or an empty string on error.
func (q *Quoter) Quotes(val string) string {
	if q.err != nil {
		return ""
	}

	switch q.Dialect {
	case SQLite, Postgres:
		return `"` + strings.ReplaceAll(val, `"`, `""`) + `"`
	case Mysql:
		return "`" + strings.ReplaceAll(val, "`", "``") + "`"
	}

	q.err = errors.New("krios: invalid dialect")

	return ""
}

// Err returns the first error encountered by Quotes.
func (q *Quoter) Err() error {
	return q.err
}
