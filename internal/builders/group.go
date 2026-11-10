package builders

import (
	"errors"
	"strings"
)

type GroupBy struct {
	Config Config
	Column string
	Table  string
}

func (g *GroupBy) buildTable(w *strings.Builder) {
	if g.Table != "" {
		w.WriteString(g.Config.Dialect.Quotes(g.Table))
		w.WriteString(".")
	}
}

func (g *GroupBy) buildColumn(w *strings.Builder) {
	w.WriteString(g.Config.Dialect.Quotes(g.Column))
}

func (g *GroupBy) Validate() error {
	if g.Config == (Config{}) {
		return errors.New("Krios - Group Builder: config is required!")
	}

	if g.Column == "" {
		return errors.New("Krios - Group Builder: column is required!")
	}

	return nil
}

func (g *GroupBy) Build() string {
	w := strings.Builder{}
	g.buildTable(&w)
	g.buildColumn(&w)
	return w.String()
}
