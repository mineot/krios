package dialects

type Dialect string

const (
	SQLite3    Dialect = "sqlite3"
	PostgreSQL Dialect = "postgres"
	MySQL      Dialect = "mysql"
)

// Renderer emits individual SQL tokens. Identifier components are validated
// by the caller; Placeholder receives a positive, one-based position.
type Renderer interface {
	QuoteIdentifier(string) string
	Placeholder(int) string
}

func Lookup(d Dialect) (Renderer, bool) {
	switch d {
	case SQLite3:
		return sqlite{}, true

	case PostgreSQL:
		return postgresql{}, true

	case MySQL:
		return mysql{}, true

	default:
		return nil, false
	}
}
