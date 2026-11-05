package dialects

import "strings"

type mysql struct{}

func (mysql) QuoteIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func (mysql) Placeholder(int) string {
	return "?"
}
