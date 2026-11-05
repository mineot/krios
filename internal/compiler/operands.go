package compiler

import (
	"strings"

	"github.com/mineot/krios/internal/expressions"
)

// Operand emits an identifier or aggregate. alias is an optional literal table
// alias for simple names, chosen by the enclosing builder's scope.
func (c *Compiler) Operand(operand any, alias string) {
	if c.err != nil {
		return
	}

	switch value := operand.(type) {
	case string:
		c.reference(expressions.NewReference(c.owner, value), alias)

	case *expressions.Reference:
		c.reference(value, alias)

	case *expressions.Expression:
		if c.active[value] {
			c.fail("cyclic expression")
			return
		}

		function, child, err := expressions.ReadExpression(value, c.owner)

		if err != nil {
			c.err = err
			return
		}

		if c.active == nil {
			c.active = make(map[*expressions.Expression]bool)
		}

		c.active[value] = true

		defer delete(c.active, value)

		c.Write(function + "(")
		c.Operand(child, alias)
		c.Write(")")

	default:
		c.fail("unsupported SQL operand")
	}
}

func (c *Compiler) reference(ref *expressions.Reference, alias string) {
	parts, wildcard, err := expressions.ReadReference(ref, c.owner)

	if err != nil {
		c.err = err
		return
	}

	if len(parts) == 1 && !wildcard && alias != "" {
		if strings.ContainsRune(alias, 0) {
			c.fail("invalid alias")
			return
		}

		c.Write(c.dialect.QuoteIdentifier(alias))
		c.Write(".")
	}

	for i, part := range parts {
		if i > 0 {
			c.Write(".")
		}

		if wildcard && i == len(parts)-1 {
			c.Write("*")
		} else {
			c.Write(c.dialect.QuoteIdentifier(part))
		}
	}
}
