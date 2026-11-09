package selectbuilder

import (
	"strings"

	"github.com/mineot/krios/internal/dialects"
)

func (s *Select) getColumns(d dialects.Dialect) string {
	if s.hasColumns() == false {
		return "*"
	}

	columns := []string{}

	for _, col := range s.Columns {
		c1 := col.Build(d)

		if col.GetError() != nil {
			s.err = col.GetError()
			return ""
		}

		columns = append(columns, c1)
	}

	return strings.Join(columns, ", ")
}

func (s *Select) hasColumns() bool {
	return len(s.Columns) > 0
}
