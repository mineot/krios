package builders

import (
	"errors"
	"strings"

	"github.com/mineot/krios/internal/core"
)

type Table struct {
	As     string
	Config core.Config
	Name   string
	Schema string

	err error
}

func (t *Table) validate() bool {
	if t.Name == "" {
		t.err = errors.New("Krios - Table Builder: table name is required!")
		return false
	}

	return true
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

func (t *Table) Err() error {
	return t.err
}

func (t *Table) Build(w *strings.Builder) {
	if t.validate() {
		t.buildSchema(w)
		t.buildName(w)
		t.buildAlias(w)
	}
}
