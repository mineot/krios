package selectbuilder

import (
	"errors"
	"strings"

	"github.com/mineot/krios/internal/dialects"
)

func (s *Select) getTables(d dialects.Dialect) string {
	if s.hasTables() == false {
		s.err = errors.New("Krios: table is required!")
		return ""
	}

	tables := []string{}

	for _, tb := range s.Tables {
		t1 := tb.Build(d)

		if tb.GetError() != nil {
			s.err = tb.GetError()
			return ""
		}

		tables = append(tables, t1)
	}

	return strings.Join(tables, ", ")
}

func (s *Select) hasTables() bool {
	return len(s.Tables) > 0
}
