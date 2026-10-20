package entity

import (
	"fmt"
	"strings"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

// Expr is a selectable expression: a column or an aggregate. Expressions are
// appended to a SELECT list with Select.Fields and, except for Sort, may carry
// an explicit output alias.
type Expr interface {
	// ExprInto renders the expression, appending bound parameters to args.
	ExprInto(d dialect.Dialect, w *strings.Builder, args *[]any) error
	// Entities returns the entities referenced by the expression.
	Entities() []*Entity
	// Alias returns the explicit output alias, or "" when the expression is
	// selected under its own name.
	Alias() string
}

// Name returns the mapped column name.
func (c Column) Name() string { return c.name }

// Alias returns "" for a column: columns are selected under their own name.
func (c Column) Alias() string { return "" }

// Entities returns the entity that owns the column.
func (c Column) Entities() []*Entity {
	if c.entity == nil {
		return nil
	}

	return []*Entity{c.entity}
}

// ExprInto renders the qualified column reference.
func (c Column) ExprInto(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	if d == nil {
		return fmt.Errorf("%w: expression requires a dialect", errs.ErrInvalidQuery)
	}

	return c.ref().render(d, w, args)
}

// Aggregate is an aggregate expression over a column, or COUNT(*) when built
// by the package-level Count. Use As to give it an output alias so results can
// be mapped by name.
type Aggregate struct {
	op    string
	col   *Column
	alias string
}

// Count returns COUNT(*), the number of rows in the group.
func Count() Aggregate {
	return Aggregate{op: "COUNT"}
}

// Count returns COUNT(column): the number of non-NULL values of the column.
func (c Column) Count() Aggregate { return Aggregate{op: "COUNT", col: &c} }

// Sum returns SUM(column).
func (c Column) Sum() Aggregate { return Aggregate{op: "SUM", col: &c} }

// Max returns MAX(column).
func (c Column) Max() Aggregate { return Aggregate{op: "MAX", col: &c} }

// Min returns MIN(column).
func (c Column) Min() Aggregate { return Aggregate{op: "MIN", col: &c} }

// As sets the output alias of the aggregate, used to map the result by name.
// An alias made only of whitespace is rejected when the query is rendered.
func (a Aggregate) As(alias string) Aggregate {
	a.alias = alias
	return a
}

// Alias returns the explicit output alias, or "" when none was set.
func (a Aggregate) Alias() string { return a.alias }

// Entities returns the entity of the aggregated column, if any.
func (a Aggregate) Entities() []*Entity {
	if a.col == nil {
		return nil
	}

	return a.col.Entities()
}

// ExprInto renders OP(column) or COUNT(*).
func (a Aggregate) ExprInto(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	if d == nil {
		return fmt.Errorf("%w: expression requires a dialect", errs.ErrInvalidQuery)
	}

	if a.op == "" || (a.col == nil && a.op != "COUNT") {
		return fmt.Errorf("%w: empty aggregate expression", errs.ErrInvalidQuery)
	}

	w.WriteString(a.op)
	w.WriteByte('(')

	if a.col != nil {
		if err := a.col.ExprInto(d, w, args); err != nil {
			return err
		}
	} else {
		w.WriteByte('*')
	}

	w.WriteByte(')')
	return nil
}

// Sort is one ORDER BY criterion: an expression and a direction. Build it with
// Asc or Desc on a Column or Aggregate.
type Sort struct {
	expr Expr
	desc bool
}

// Asc orders by the column in ascending order.
func (c Column) Asc() Sort { return Sort{expr: c} }

// Desc orders by the column in descending order.
func (c Column) Desc() Sort { return Sort{expr: c, desc: true} }

// Asc orders by the aggregate in ascending order.
func (a Aggregate) Asc() Sort { return Sort{expr: a} }

// Desc orders by the aggregate in descending order.
func (a Aggregate) Desc() Sort { return Sort{expr: a, desc: true} }

// Entities returns the entities referenced by the sort expression.
func (s Sort) Entities() []*Entity {
	if s.expr == nil {
		return nil
	}

	return s.expr.Entities()
}

// ExprInto renders "expression ASC" or "expression DESC".
func (s Sort) ExprInto(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	if d == nil {
		return fmt.Errorf("%w: expression requires a dialect", errs.ErrInvalidQuery)
	}

	if s.expr == nil {
		return fmt.Errorf("%w: empty sort criterion", errs.ErrInvalidQuery)
	}

	if err := s.expr.ExprInto(d, w, args); err != nil {
		return err
	}

	if s.desc {
		w.WriteString(" DESC")
	} else {
		w.WriteString(" ASC")
	}

	return nil
}
