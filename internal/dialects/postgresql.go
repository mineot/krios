package dialects

import (
	"strconv"
	"strings"
)

type postgresql struct{}

func (postgresql) QuoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func (postgresql) Placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}
