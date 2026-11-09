package builder

import (
	"errors"

	"github.com/mineot/krios/internal/dialects"
)

type Operator string

const (
	Eq         Operator = "="
	Ne         Operator = "<>"
	Gt         Operator = ">"
	Gte        Operator = ">="
	Lt         Operator = "<"
	Lte        Operator = "<="
	In         Operator = "IN"
	NotIn      Operator = "NOT IN"
	IsNull     Operator = "IS NULL"
	IsNotNull  Operator = "IS NOT NULL"
	Like       Operator = "LIKE"
	NotLike    Operator = "NOT LIKE"
	Between    Operator = "BETWEEN"
	NotBetween Operator = "NOT BETWEEN"
)

type Expression struct {
	Column   string
	Operator Operator
	Table    string
	Value    any
	err      error
}

func (e *Expression) GetError() error {
	return e.err
}

func (e *Expression) hasColumn() bool {
	return e.Column != ""
}

func (e *Expression) hasOperator() bool {
	return e.Operator != ""
}

func (e *Expression) hasTable() bool {
	return e.Table != ""
}

func (e *Expression) hasValue() bool {
	return e.Value != nil
}

func (e *Expression) getColumn(d dialects.Dialect) string {
	return d.Quotes(e.Column)
}

func (e *Expression) getTable(d dialects.Dialect) string {
	return d.Quotes(e.Table)
}

// func (e *Expression) getPlaceHolder(d dialects.Dialect, index uint) string {
// 	return d.Placeholder(index)
// }

func (e *Expression) Build(d dialects.Dialect, index uint) string {
	if e.hasColumn() == false {
		e.err = errors.New("Krios Predicate: column name is required!")
		return ""
	}

	if e.hasOperator() == false {
		e.err = errors.New("Krios Predicate: operator is required!")
		return ""
	}

	predicate := e.getColumn(d)

	if e.hasTable() {
		predicate = e.getTable(d) + "." + predicate
	}

	// predicate = predicate + " " + e.Operator

	// if e.hasValue() {
	// 	predicate = predicate + " " + e.getPlaceHolder(d, index)
	// }

	return predicate
}
