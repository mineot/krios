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
	// safe to run inside a transaction. It is conservative (C03-P02 D10): a
	// statement-leading keyword outside the transactional allowlist is
	// treated as non-transactional ("in doubt, false"). A transaction
	// control statement (BEGIN, COMMIT, ROLLBACK, SAVEPOINT, ...) is
	// rejected with an error because the library owns the migration scope
	// (C03-P02 D11). An unterminated block comment is also an error.
	IsTransactionalStatement(sql string) (bool, error)
}

// nonTransactionalKeywords are the statement-leading keywords that cause an
// implicit commit or are otherwise known to be unsafe inside a transaction on
// MySQL. The list was checked against the official MySQL reference manual,
// §15.3.3 "Statements That Cause an Implicit Commit" (C03-P02 D10): DDL,
// statements of the mysql system tables (users/privileges), locking, data
// loading and administrative statements.
var nonTransactionalKeywords = map[string]bool{
	"CREATE":   true,
	"ALTER":    true,
	"DROP":     true,
	"RENAME":   true,
	"TRUNCATE": true,
	"LOCK":     true,
	"UNLOCK":   true,
	"GRANT":    true,
	"REVOKE":   true,
	"LOAD":     true,
	"CHECK":    true,
	"CACHE":    true,
	"RESET":    true,
	"REPAIR":   true,
	"OPTIMIZE": true,
	"ANALYZE":  true,
	"FLUSH":    true,
}

// transactionalKeywords are the statement-leading keywords known to be safe
// inside a transaction (DML on InnoDB does not cause an implicit commit).
// Everything outside this set and the non-transactional set is classified as
// non-transactional ("in doubt, false"). SET is deliberately absent: SET
// PASSWORD, SET autocommit and SET TRANSACTION cause an implicit commit and
// share the leading keyword with harmless statements such as SET NAMES
// (C03-P02 D10 and §1).
var transactionalKeywords = map[string]bool{
	"INSERT":  true,
	"UPDATE":  true,
	"DELETE":  true,
	"SELECT":  true,
	"REPLACE": true,
	"WITH":    true,
}

// controlKeywords are the statement-leading keywords that manage transactions
// or replication. The library owns the transaction scope of migrations
// (E08-P01 §11), so a consumer-supplied control statement is rejected before
// anything is applied (C03-P02 D11).
var controlKeywords = map[string]bool{
	"BEGIN":     true,
	"START":     true,
	"COMMIT":    true,
	"ROLLBACK":  true,
	"SAVEPOINT": true,
	"RELEASE":   true,
	"END":       true,
	"ABORT":     true,
}

func (sqlite) SupportsTransactionalDDL() bool   { return true }
func (mysql) SupportsTransactionalDDL() bool    { return false }
func (postgres) SupportsTransactionalDDL() bool { return true }

// isTransactionalStatement strips leading comments and spaces, then checks the
// first keyword: control statements are rejected with an error, known
// non-transactional keywords return false, the transactional allowlist
// returns true and anything else returns false ("in doubt, false"). An empty
// statement (or only comments) has nothing that could commit implicitly. An
// unterminated block comment is reported as an error instead of guessing. A
// first token that starts with a quote character matches no list and is
// therefore treated as non-transactional, without any quote parsing
// (C03-P02 D13).
func isTransactionalStatement(sql string) (bool, error) {
	keyword, ok, err := leadingKeyword(sql)

	if err != nil {
		return false, err
	}

	if !ok {
		return true, nil
	}

	upper := strings.ToUpper(keyword)

	if controlKeywords[upper] {
		return false, fmt.Errorf("transaction control statement %q is not allowed in migration files", upper)
	}

	if nonTransactionalKeywords[upper] {
		return false, nil
	}

	return transactionalKeywords[upper], nil
}

// leadingKeyword returns the first meaningful token of sql after stripping
// line and block comments and horizontal/vertical space. The second result is
// false when nothing but whitespace or comments remains; the third reports an
// unterminated block comment, which is malformed input.
func leadingKeyword(sql string) (string, bool, error) {
	s := sql

	for {
		trimmed := strings.TrimLeft(s, " \t\r\n")

		if trimmed == "" {
			return "", false, nil
		}

		if strings.HasPrefix(trimmed, "--") {
			i := strings.IndexByte(trimmed, '\n')

			if i < 0 {
				return "", false, nil
			}

			s = trimmed[i+1:]
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			i := strings.Index(trimmed, "*/")

			if i < 0 {
				return "", false, errUnterminatedComment
			}

			s = trimmed[i+2:]
			continue
		}

		keyword := trimmed

		if i := strings.IndexAny(keyword, " \t\r\n(;"); i >= 0 {
			keyword = keyword[:i]
		}

		return keyword, true, nil
	}
}

// errUnterminatedComment reports a block comment that is never closed. The
// file is malformed, so preparation fails instead of guessing a scope.
var errUnterminatedComment = fmt.Errorf("unterminated block comment")

func (sqlite) IsTransactionalStatement(sql string) (bool, error) {
	return isTransactionalStatement(sql)
}

func (mysql) IsTransactionalStatement(sql string) (bool, error) {
	return isTransactionalStatement(sql)
}

func (postgres) IsTransactionalStatement(sql string) (bool, error) {
	return isTransactionalStatement(sql)
}
