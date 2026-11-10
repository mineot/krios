package builders

import (
	"errors"
	"strings"
)

type Table struct {
	As     string
	Config Config
	Name   string
	Schema string
}

func (t *Table) buildSchema(w *strings.Builder) {
	if t.Schema != "" {
		w.WriteString(t.Config.Dialect.Quotes(t.Schema))
		w.WriteString(".")
	}
}

func (t *Table) buildName(w *strings.Builder) {
	w.WriteString(t.Config.Dialect.Quotes(t.Name))
}

func (t *Table) buildAlias(w *strings.Builder) {
	if t.As != "" {
		w.WriteString(" AS ")
		w.WriteString(t.Config.Dialect.Quotes(t.As))
	}
}

func (t *Table) Validate() error {
	if t.Config == (Config{}) {
		return errors.New("Krios - Table Builder: table config is required!")
	}

	if t.Name == "" {
		return errors.New("Krios - Table Builder: table name is required!")
	}

	return nil
}

func (t *Table) Build() string {
	w := &strings.Builder{}
	t.buildSchema(w)
	t.buildName(w)
	t.buildAlias(w)
	return w.String()
}
