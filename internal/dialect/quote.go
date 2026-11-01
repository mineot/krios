package dialect

import (
	"fmt"
	"strings"

	"github.com/mineot/krios/internal/errs"
)

// QuoteIdentifier delimits one structurally validated identifier component.
// Qualification and projection wildcards are handled by the AST renderer.
func QuoteIdentifier(kind Dialect, name string) (string, error) {
	var quote string
	switch kind {
	case Postgres, SQLite:
		quote = "\""
	case MySQL:
		quote = "`"
	default:
		return "", fmt.Errorf("%w: unknown dialect", errs.ErrInvalidConfig)
	}

	return quote + strings.ReplaceAll(name, quote, quote+quote) + quote, nil
}
