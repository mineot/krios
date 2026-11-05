package core

import "github.com/mineot/krios/internal/criteria"

// In creates a client-owned In criterion.
func (c *Client) In(operand, values any) *criteria.Criterion {
	return criteria.In(c.identity, operand, values)
}

// NotIn creates a client-owned NotIn criterion.
func (c *Client) NotIn(operand, values any) *criteria.Criterion {
	return criteria.NotIn(c.identity, operand, values)
}
