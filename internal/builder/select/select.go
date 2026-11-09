package selectbuilder

import (
	"strings"

	"github.com/mineot/krios/internal/builder"
	"github.com/mineot/krios/internal/dialects"
)

type Select struct {
	Columns []builder.Column
	Tables  []builder.Table
	Groups  []builder.GroupBy
	Orders  []builder.OrderBy
	// quando fizermos o where vamos fazer o having

	errors []error
}

func (s *Select) appendError(err error) {
	s.errors = append(s.errors, err)
}

func (s *Select) HasErrors() bool {
	return len(s.errors) > 0
}

func (s *Select) GetErrors() []error {
	return s.errors
}

func (s *Select) Build(d dialects.Dialect) string {
	w := strings.Builder{}

	w.WriteString("SELECT ")
	w.WriteString(s.getColumns(d))
	w.WriteString(" FROM ")
	w.WriteString(s.getTables(d))

	if s.hasGroups() {
		w.WriteString(" GROUP BY ")
		w.WriteString(s.getGroups(d))
	}

	if s.hasOrders() {
		w.WriteString(" ORDER BY ")
		w.WriteString(s.getOrders(d))
	}

	return w.String()
}
