package builder

import (
	"errors"

	"github.com/mineot/krios/internal/dialects"
)

type GroupBy struct {
	Name  string
	Table string
	err   error
}

func (g *GroupBy) GetError() error {
	return g.err
}

func (g *GroupBy) hasName() bool {
	return g.Name != ""
}

func (g *GroupBy) getName(d dialects.Dialect) string {
	return d.Quotes(g.Name)
}

func (g *GroupBy) hasTable() bool {
	return g.Table != ""
}

func (g *GroupBy) getTable(d dialects.Dialect) string {
	return d.Quotes(g.Table)
}

func (g *GroupBy) Build(d dialects.Dialect) string {
	if g.hasName() == false {
		g.err = errors.New("Krios Group By: column name is required!")
		return ""
	}

	column := g.getName(d)

	if g.hasTable() {
		column = g.getTable(d) + "." + column
	}

	return column
}
