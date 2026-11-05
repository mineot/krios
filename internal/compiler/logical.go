package compiler

import "github.com/mineot/krios/internal/criteria"

func (c *Compiler) logical(node criteria.Snapshot, alias string) {
	c.Write("(")
	for i, child := range node.Children {
		if c.err != nil {
			return
		}
		if i > 0 {
			c.Write(" " + string(node.Operator) + " ")
		}
		c.Criterion(child, alias)
	}
	c.Write(")")
}
