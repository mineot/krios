package builders

import (
	"errors"
	"strings"
)

type Operator string

const (
	Eq         Operator = "="
	Ne         Operator = "!="
	Lt         Operator = "<"
	Gt         Operator = ">"
	Lte        Operator = "<="
	Gte        Operator = ">="
	Like       Operator = "LIKE"
	NotLike    Operator = "NOT LIKE"
	IsNull     Operator = "IS NULL"
	IsNotNull  Operator = "IS NOT NULL"
	In         Operator = "IS"
	NotIn      Operator = "IS NOT"
	Between    Operator = "BETWEEN"
	NotBetween Operator = "NOT BETWEEN"
)

type Criteria struct {
	Column   string
	Config   Config
	Index    int
	Operator Operator
	Or       []Criteria
	Table    string
	Value    any
}

func (c *Criteria) Validate() error {
	if c.Config == (Config{}) {
		return errors.New("Krios - Criteria Builder: criteria config is required!")
	}

	if c.Column == "" {
		return errors.New("Krios - Criteria Builder: column is required!")
	}

	if c.Operator == "" {
		return errors.New("Krios - Criteria Builder: operator is required!")
	}

	if c.Operator != IsNull || c.Operator != IsNotNull && c.Value == nil {
		return errors.New("Krios - Criteria Builder: value is required!")
	}

	return nil
}

func (c *Criteria) buildTable(w *strings.Builder) {
	if c.Table != "" {
		w.WriteString(c.Config.Dialect.Quotes(c.Table))
		w.WriteString(".")
	}
}

func (c *Criteria) buildColumn(w *strings.Builder) {
	w.WriteString(c.Config.Dialect.Quotes(c.Column))
}

func (c *Criteria) buildOperator(w *strings.Builder) {
	w.WriteString(" ")
	w.WriteString(string(c.Operator))
	w.WriteString(" ")
}

func (c *Criteria) buildPlaceholder(w *strings.Builder) {
	w.WriteString(c.Config.Dialect.Placeholder(c.Index))
}

func (c *Criteria) build() string {
	w := strings.Builder{}
	c.buildTable(&w)
	c.buildColumn(&w)
	c.buildOperator(&w)
	c.buildPlaceholder(&w)
	return w.String()
}

func (c *Criteria) Build() string {
	if len(c.Or) > 0 {
		var ors []string = []string{c.build()}

		for _, or := range c.Or {
			ors = append(ors, or.build())
		}

		return "(" + strings.Join(ors, " OR ") + ")"
	}

	return c.build()
}

func (c *Criteria) Values() []any {
	if len(c.Or) > 0 {
		var vs []any = []any{c.Value}

		for _, or := range c.Or {
			vs = append(vs, or.Value)
		}

		return vs
	}

	return []any{c.Value}
}
