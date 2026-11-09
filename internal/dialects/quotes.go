package dialects

import (
	"strings"
)

func (d *Dialect) Quotes(val string) string {
	switch *d {
	case SQLite, Postgres:
		return d.quoteConcat(`"`, `""`, val)
	case Mysql:
		return d.quoteConcat("`", "``", val)
	}

	return val
}

func (d *Dialect) quoteConcat(quote, doubleQuote, val string) string {
	return quote + strings.ReplaceAll(val, quote, doubleQuote) + quote
}
