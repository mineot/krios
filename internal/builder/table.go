package builder

import (
	"errors"

	"github.com/mineot/krios/internal/dialects"
)

type Table struct {
	As     string
	Name   string
	Schema string
	err    error
}

func (t *Table) GetError() error {
	return t.err
}

func (t *Table) getName(d dialects.Dialect) string {
	return d.Quotes(t.Name)
}

func (t *Table) hasName() bool {
	return t.Name != ""
}

func (t *Table) getAs(d dialects.Dialect) string {
	return d.Quotes(t.As)
}

func (t *Table) hasAs() bool {
	return t.As != ""
}

func (t *Table) getSchema(d dialects.Dialect) string {
	return d.Quotes(t.Schema)
}

func (t *Table) hasSchema() bool {
	return t.Schema != ""
}

func (t *Table) Build(d dialects.Dialect) string {
	if t.hasName() == false {
		t.err = errors.New("Krios Table: table name is required!")
		return ""
	}

	table := t.getName(d)

	if t.hasSchema() {
		table = t.getSchema(d) + "." + table
	}

	if t.hasAs() {
		table = table + " AS " + t.getAs(d)
	}

	return table
}
