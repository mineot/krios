package core

import "github.com/mineot/krios/internal/criteria"

// And creates a client-owned And criterion.
func (c *Client) And(children ...*criteria.Criterion) *criteria.Criterion {
	return criteria.And(c.identity, children...)
}

// Or creates a client-owned Or criterion.
func (c *Client) Or(children ...*criteria.Criterion) *criteria.Criterion {
	return criteria.Or(c.identity, children...)
}
