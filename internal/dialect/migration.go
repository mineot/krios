package dialect

import (
	"fmt"
	"strings"
)

// MigrationDialect reports the transactional capabilities of a dialect for
// migrations (E08-P01). Every concrete dialect type implements it in addition
// to Dialect; migration code consumes it through this interface.
type MigrationDialect interface {
	// SupportsTransactionalDDL reports whether all DDL is transactional in
	// this dialect.
	SupportsTransactionalDDL() bool

	// IsTransactionalStatement reports whether the statement is known to be
	// safe to run inside a transaction. It must be conservative: when in
	// doubt, return false.
	IsTransactionalStatement(sql string) (bool, error)
}

// nonTransactionalKeywords are the statement-leading keywords that justify
// treating a statement as non-transactional (E08-P01 §3): in doubt, false.
var nonTransactionalKeywords = map[string]bool{
	"CREATE":   true,
	"ALTER":    true,
	"DROP":     true,
	"RENAME":   true,
	"TRUNCATE": true,
	"LOCK":     true,
	"UNLOCK":   true,
	"REPAIR":   true,
	"OPTIMIZE": true,
	"ANALYZE":  true,
	"FLUSH":    true,
}

func (sqlite) SupportsTransactionalDDL() bool   { return true }
func (mysql) SupportsTransactionalDDL() bool    { return false }
func (postgres) SupportsTransactionalDDL() bool { return true }

// IsTransactionalStatement strips leading comments and spaces, then checks the
// first keyword against the conservative list. An empty statement (or only
// comments) has nothing that could commit implicitly. An unterminated block
// comment is reported as an error instead of guessing.
func isTransactionalStatement(sql string) (bool, error) {
	s := sql
	for {
		trimmed := strings.TrimLeft(s, " \t\r\n")

		if trimmed == "" {
			return true, nil
		}

		if strings.HasPrefix(trimmed, "--") {
			i := strings.IndexByte(trimmed, '\n')

			if i < 0 {
				return true, nil
			}

			s = trimmed[i+1:]
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			i := strings.Index(trimmed, "*/")

			if i < 0 {
				return false, fmt.Errorf("unterminated block comment")
			}

			s = trimmed[i+2:]
			continue
		}

		s = trimmed
		break
	}

	keyword := s

	if i := strings.IndexAny(keyword, " \t\r\n(;"); i >= 0 {
		keyword = keyword[:i]
	}

	return !nonTransactionalKeywords[strings.ToUpper(keyword)], nil
}

func (sqlite) IsTransactionalStatement(sql string) (bool, error) {
	return isTransactionalStatement(sql)
}

func (mysql) IsTransactionalStatement(sql string) (bool, error) {
	return isTransactionalStatement(sql)
}

func (postgres) IsTransactionalStatement(sql string) (bool, error) {
	return isTransactionalStatement(sql)
}
