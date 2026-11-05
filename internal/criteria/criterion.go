package criteria

import (
	"fmt"
	"slices"

	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/expressions"
)

type Operator string

const (
	Equal              Operator = "="
	NotEqual           Operator = "!="
	Less               Operator = "<"
	LessEqual          Operator = "<="
	Greater            Operator = ">"
	GreaterEqual       Operator = ">="
	LikeOperator       Operator = "LIKE"
	NotLikeOperator    Operator = "NOT LIKE"
	Null               Operator = "IS NULL"
	NotNull            Operator = "IS NOT NULL"
	InOperator         Operator = "IN"
	NotInOperator      Operator = "NOT IN"
	BetweenOperator    Operator = "BETWEEN"
	NotBetweenOperator Operator = "NOT BETWEEN"
	AndOperator        Operator = "AND"
	OrOperator         Operator = "OR"
)

// Criterion describes a predicate owned by one client. Its components have no
// public setters and are retained by reference unless their sequence is copied.
type Criterion struct {
	owner    *expressions.Identity
	operator Operator
	left     any
	right    any
	upper    any
	values   []any
	hasList  bool
	children []*Criterion
	err      error
}

// Snapshot is an internal view. Sequence slices are copied; individual operand
// values and child criteria retain their original references.
type Snapshot struct {
	Operator           Operator
	Left, Right, Upper any
	Values             []any
	HasList            bool
	Children           []*Criterion
}

func Read(c *Criterion, owner *expressions.Identity) (Snapshot, error) {
	if err := validate(c, owner); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Operator: c.operator, Left: c.left, Right: c.right, Upper: c.upper, Values: slices.Clone(c.values), HasList: c.hasList, Children: slices.Clone(c.children)}, nil
}
func validate(c *Criterion, owner *expressions.Identity) error {
	if c == nil || !expressions.ValidIdentity(c.owner) || !expressions.ValidIdentity(owner) {
		return invalid("invalid criterion")
	}
	if c.err != nil {
		return c.err
	}
	if c.owner != owner {
		return invalid("criterion belongs to another client")
	}
	return nil
}
func newCriterion(owner *expressions.Identity, operator Operator) *Criterion {
	c := &Criterion{owner: owner, operator: operator}
	if !expressions.ValidIdentity(owner) {
		c.err = invalid("invalid criterion origin")
	}
	return c
}
func predicate(owner *expressions.Identity, operator Operator, left any) *Criterion {
	c := newCriterion(owner, operator)
	if name, ok := left.(string); ok {
		left = expressions.NewReference(owner, name)
	}
	c.left = left
	c.record(validateOperand(left, owner))
	return c
}
func validateOperand(value any, owner *expressions.Identity) error {
	switch v := value.(type) {
	case *expressions.Reference:
		_, _, err := expressions.ReadReference(v, owner)
		return err
	case *expressions.Expression:
		_, _, err := expressions.ReadExpression(v, owner)
		return err
	default:
		return invalid("unsupported predicate operand")
	}
}
func validateValue(value any, owner *expressions.Identity) error {
	switch value.(type) {
	case *expressions.Reference, *expressions.Expression:
		return validateOperand(value, owner)
	default:
		return nil
	}
}
func (c *Criterion) record(err error) {
	if c.err == nil {
		c.err = err
	}
}
func invalid(message string) error { return fmt.Errorf("%w: %s", errs.ErrInvalidQuery, message) }
