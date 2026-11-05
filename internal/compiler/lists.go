package compiler

import "github.com/mineot/krios/internal/criteria"

func (c *Compiler) list(node criteria.Snapshot, alias string) {
	if !node.HasList {
		c.fail("IN requires a list or subquery")
		return
	}
	c.Operand(node.Left, alias)
	c.Write(" " + string(node.Operator) + " (")
	for i, value := range node.Values {
		if c.err != nil {
			return
		}
		if i > 0 {
			c.Write(", ")
		}
		c.Parameter(value)
	}
	c.Write(")")
}
