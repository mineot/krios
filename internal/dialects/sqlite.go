package dialects

import "strings"

type sqlite struct{}

func (sqlite) QuoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func (sqlite) Placeholder(int) string {
	return "?"
}
