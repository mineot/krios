package compiler

import (
	"github.com/mineot/krios/internal/criteria"
	"github.com/mineot/krios/internal/expressions"
)

// Criterion appends a predicate using the current command's parameter sequence.
// alias is chosen by the enclosing builder, as with Operand.
func (c *Compiler) Criterion(criterion *criteria.Criterion, alias string) {
	if c.err != nil {
		return
	}
	if c.activeCriteria[criterion] {
		c.fail("cyclic criterion")
		return
	}
	node, err := criteria.Read(criterion, c.owner)
	if err != nil {
		c.err = err
		return
	}
	if c.activeCriteria == nil {
		c.activeCriteria = make(map[*criteria.Criterion]bool)
	}
	c.activeCriteria[criterion] = true
	defer delete(c.activeCriteria, criterion)
	switch node.Operator {
	case criteria.AndOperator, criteria.OrOperator:
		c.logical(node, alias)
	case criteria.Equal, criteria.NotEqual, criteria.Less, criteria.LessEqual, criteria.Greater, criteria.GreaterEqual, criteria.LikeOperator, criteria.NotLikeOperator:
		c.Operand(node.Left, alias)
		c.Write(" " + string(node.Operator) + " ")
		c.criterionValue(node.Right, alias)
	case criteria.Null, criteria.NotNull:
		c.Operand(node.Left, alias)
		c.Write(" " + string(node.Operator))
	case criteria.InOperator, criteria.NotInOperator:
		c.list(node, alias)
	case criteria.BetweenOperator, criteria.NotBetweenOperator:
		c.Operand(node.Left, alias)
		c.Write(" " + string(node.Operator) + " ")
		c.criterionValue(node.Right, alias)
		c.Write(" AND ")
		c.criterionValue(node.Upper, alias)
	default:
		c.fail("unsupported criterion operator")
	}
}
func (c *Compiler) criterionValue(value any, alias string) {
	switch value.(type) {
	case *expressions.Reference, *expressions.Expression:
		c.Operand(value, alias)
	default:
		c.Parameter(value)
	}
}
