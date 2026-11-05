package core

import "github.com/mineot/krios/internal/criteria"

// IsNull creates a client-owned IsNull criterion.
func (c *Client) IsNull(operand any) *criteria.Criterion { return criteria.IsNull(c.identity, operand) }

// IsNotNull creates a client-owned IsNotNull criterion.
func (c *Client) IsNotNull(operand any) *criteria.Criterion {
	return criteria.IsNotNull(c.identity, operand)
}
