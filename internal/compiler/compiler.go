package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mineot/krios/internal/criteria"
	"github.com/mineot/krios/internal/dialects"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/expressions"
)

// Compiler owns the mutable state of one SQL command. Do not share an instance
// between concurrent compilations. Operands can be reused by independent instances.
type Compiler struct {
	dialect        dialects.Renderer
	owner          *expressions.Identity
	sql            strings.Builder
	args           []any
	err            error
	active         map[*expressions.Expression]bool
	activeCriteria map[*criteria.Criterion]bool
}

func New(dialect dialects.Dialect, owner *expressions.Identity) *Compiler {
	renderer, ok := dialects.Lookup(dialect)
	c := &Compiler{dialect: renderer, owner: owner}

	if !ok {
		c.fail("unsupported dialect")
	}

	if !expressions.ValidIdentity(owner) {
		c.fail("invalid command origin")
	}

	return c
}

// Write appends trusted syntax supplied by internal builders, never user SQL.
func (c *Compiler) Write(syntax string) {
	if c.err == nil {
		c.sql.WriteString(syntax)
	}
}

func (c *Compiler) Result() (string, []any, error) {
	if c.err != nil {
		return "", nil, c.err
	}

	return c.sql.String(), slices.Clone(c.args), nil
}

func (c *Compiler) fail(message string) {
	if c.err == nil {
		c.err = fmt.Errorf("%w: %s", errs.ErrInvalidQuery, message)
	}
}
