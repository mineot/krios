package builders

import (
	"errors"
	"strings"
)

type TableBuilder struct {
	As     string
	Name   string
	Schema string

	config ConfigBuilder
	err    error
}

func (t *TableBuilder) validate() bool {
	if t.Name == "" {
		t.err = errors.New("Krios - Table Builder: table name is required!")
		return false
	}

	return true
}

func (t *TableBuilder) buildSchema(w *strings.Builder) {
	if t.Schema != "" {
		w.WriteString(t.config.Dialect.Quotes(t.Schema))
		w.WriteString(".")
	}
}

func (t *TableBuilder) buildName(w *strings.Builder) {
	w.WriteString(t.config.Dialect.Quotes(t.Name))
}

func (t *TableBuilder) buildAlias(w *strings.Builder) {
	if t.As != "" {
		w.WriteString(" AS ")
		w.WriteString(t.config.Dialect.Quotes(t.As))
	}
}

func (t *TableBuilder) SetConfig(config ConfigBuilder) {
	t.config = config
}

func (t *TableBuilder) Err() error {
	return t.err
}

func (t *TableBuilder) Build() string {
	if t.validate() {
		w := &strings.Builder{}
		t.buildSchema(w)
		t.buildName(w)
		t.buildAlias(w)
		return w.String()
	}

	return ""
}
