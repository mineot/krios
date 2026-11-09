package builder

import (
	"errors"

	"github.com/mineot/krios/internal/dialects"
)

type Direction string

const (
	Asc  Direction = "ASC"
	Desc Direction = "DESC"
)

type OrderBy struct {
	Name      string
	Direction Direction
	Table     string
	err       error
}

func (o *OrderBy) GetError() error {
	return o.err
}

func (o *OrderBy) hasName() bool {
	return o.Name != ""
}

func (o *OrderBy) getName(d dialects.Dialect) string {
	return d.Quotes(o.Name)
}

func (o *OrderBy) hasTable() bool {
	return o.Table != ""
}

func (o *OrderBy) getTable(d dialects.Dialect) string {
	return d.Quotes(o.Table)
}

func (o *OrderBy) isDesc() bool {
	return o.Direction == Desc
}

func (o *OrderBy) Build(d dialects.Dialect) string {
	if o.hasName() == false {
		o.err = errors.New("Krios Order By: column name is required!")
		return ""
	}

	column := o.getName(d)

	if o.hasTable() {
		column = o.getTable(d) + "." + column
	}

	if o.isDesc() {
		column = column + " DESC"
	} else {
		column = column + " ASC"
	}

	return column
}
