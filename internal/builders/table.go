package builders

import (
	"strings"

	"github.com/mineot/krios/internal/dialects"
)

type Table struct {
	Dialect dialects.Dialect
	Name    string
	alias   string
	schema  string
}

func (t *Table) As(alias string) *Table {
	t.alias = t.Dialect.Quotes(alias)
	return t
}

func (t *Table) Schema(schema string) *Table {
	t.schema = t.Dialect.Quotes(schema)
	return t
}

func (t *Table) Build() string {
	w := strings.Builder{}

	if t.schema != "" {
		w.WriteString(t.schema)
		w.WriteString(".")
	}

	w.WriteString(t.Dialect.Quotes(t.Name))

	if t.alias != "" {
		w.WriteString(" AS ")
		w.WriteString(t.alias)
	}

	return w.String()
}
