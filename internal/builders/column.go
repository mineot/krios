package builders

import (
	"errors"
	"strings"
)

type Column struct {
	Aggregate Aggregate
	As        string
	Config    Config
	Name      string
	Table     string
}

func (c *Column) buildTable(w *strings.Builder) {
	if c.Table != "" {
		w.WriteString(c.Config.Dialect.Quotes(c.Table))
		w.WriteString(".")
	}
}

func (c *Column) buildName(w *strings.Builder) {
	if c.Aggregate != "" {
		w.WriteString(string(c.Aggregate))
		w.WriteString("(")
		c.buildTable(w)
		w.WriteString(c.Config.Dialect.Quotes(c.Name))
		w.WriteString(")")
	} else {
		c.buildTable(w)
		w.WriteString(c.Config.Dialect.Quotes(c.Name))
	}
}

func (c *Column) buildAlias(w *strings.Builder) {
	if c.As != "" {
		w.WriteString(" AS ")
		w.WriteString(c.Config.Dialect.Quotes(c.As))
	}
}

func (c *Column) Validate() error {
	if c.Config == (Config{}) {
		return errors.New("Krios - Column Builder: column config is required!")
	}

	if c.Name == "" {
		return errors.New("Krios - Column Builder: column name is required!")
	}

	return nil
}

func (c *Column) Build() string {
	w := strings.Builder{}
	c.buildName(&w)
	c.buildAlias(&w)
	return w.String()
}
