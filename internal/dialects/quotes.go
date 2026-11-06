package dialects

import (
	"fmt"
	"strings"
)

func (d *Dialect) Quotes(val string) (string, error) {
	switch *d {
	case SQLite, Postgres:
		return d.quoteConcat(`"`, `""`, val), nil
	case Mysql:
		return d.quoteConcat("`", "``", val), nil
	}

	return "", fmt.Errorf(ErrInvalidDialect, *d)
}

func (d *Dialect) quoteConcat(quote, doubleQuote, val string) string {
	return quote + strings.ReplaceAll(val, quote, doubleQuote) + quote
}
