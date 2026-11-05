package criteria

import (
	"github.com/mineot/krios/internal/expressions"
	"slices"
)

func And(owner *expressions.Identity, children ...*Criterion) *Criterion {
	return group(owner, AndOperator, children)
}
func Or(owner *expressions.Identity, children ...*Criterion) *Criterion {
	return group(owner, OrOperator, children)
}
func group(owner *expressions.Identity, operator Operator, children []*Criterion) *Criterion {
	c := newCriterion(owner, operator)
	c.children = slices.Clone(children)
	if len(children) == 0 {
		c.record(invalid("logical group requires criteria"))
	}
	for _, child := range children {
		c.record(validate(child, owner))
	}
	return c
}
