package selectbuilder

import (
	"strings"

	"github.com/mineot/krios/internal/builder"
	"github.com/mineot/krios/internal/dialects"
)

type Select struct {
	Columns []builder.Column
	Tables  []builder.Table
	err     error
}

func (s *Select) GetError() error {
	return s.err
}

func (s *Select) Build(d dialects.Dialect) string {
	w := strings.Builder{}
	w.WriteString("SELECT ")
	w.WriteString(s.getColumns(d))
	w.WriteString(" FROM ")
	w.WriteString(s.getTables(d))
	return w.String()
}
