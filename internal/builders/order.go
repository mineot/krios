package builders

import (
	"errors"
	"strings"
)

type Direction string

const (
	Asc  Direction = "ASC"
	Desc Direction = "DESC"
)

type OrderBy struct {
	Column    string
	Config    Config
	Direction Direction
	Table     string
}

func (o *OrderBy) buildTable(w *strings.Builder) {
	if o.Table != "" {
		w.WriteString(o.Config.Dialect.Quotes(o.Table))
		w.WriteString(".")
	}
}

func (o *OrderBy) buildColumn(w *strings.Builder) {
	w.WriteString(o.Config.Dialect.Quotes(o.Column))
}

func (o *OrderBy) buildDirection(w *strings.Builder) {
	if o.Direction == Desc {
		w.WriteString(" DESC")
	} else {
		w.WriteString(" ASC")
	}
}

func (o *OrderBy) Validate() error {
	if o.Config == (Config{}) {
		return errors.New("Krios - Order Builder: config is required!")
	}

	if o.Column == "" {
		return errors.New("Krios - Order Builder: column is required!")
	}

	return nil
}

func (o *OrderBy) Build() string {
	w := strings.Builder{}
	o.buildTable(&w)
	o.buildColumn(&w)
	o.buildDirection(&w)
	return w.String()
}
