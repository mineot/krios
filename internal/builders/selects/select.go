package selects

import (
	"github.com/mineot/krios/internal/builders"
	"github.com/mineot/krios/internal/dialects"
)

type Select struct {
	Dialect   dialects.Dialect
	columns   []string
	froms     []builders.Table
	joins     []string
	criterias []string
	orders    []string
	groups    []string
	paginate  []string
	err       error
}

func (s *Select) From(name string) *builders.Table {
	table := builders.Table{Name: name, Dialect: s.Dialect}
	s.froms = append(s.froms, table)
	return &table
}
