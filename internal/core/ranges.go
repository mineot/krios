package core

import "github.com/mineot/krios/internal/criteria"

// Between creates a client-owned Between criterion.
func (c *Client) Between(operand, lower, upper any) *criteria.Criterion {
	return criteria.Between(c.identity, operand, lower, upper)
}

// NotBetween creates a client-owned NotBetween criterion.
func (c *Client) NotBetween(operand, lower, upper any) *criteria.Criterion {
	return criteria.NotBetween(c.identity, operand, lower, upper)
}
