package core

import "github.com/mineot/krios/internal/criteria"

// Eq creates a client-owned Eq criterion.
func (c *Client) Eq(left, right any) *criteria.Criterion { return criteria.Eq(c.identity, left, right) }

// Ne creates a client-owned Ne criterion.
func (c *Client) Ne(left, right any) *criteria.Criterion { return criteria.Ne(c.identity, left, right) }

// Lt creates a client-owned Lt criterion.
func (c *Client) Lt(left, right any) *criteria.Criterion { return criteria.Lt(c.identity, left, right) }

// Lte creates a client-owned Lte criterion.
func (c *Client) Lte(left, right any) *criteria.Criterion {
	return criteria.Lte(c.identity, left, right)
}

// Gt creates a client-owned Gt criterion.
func (c *Client) Gt(left, right any) *criteria.Criterion { return criteria.Gt(c.identity, left, right) }

// Gte creates a client-owned Gte criterion.
func (c *Client) Gte(left, right any) *criteria.Criterion {
	return criteria.Gte(c.identity, left, right)
}

// Like creates a client-owned Like criterion.
func (c *Client) Like(left, right any) *criteria.Criterion {
	return criteria.Like(c.identity, left, right)
}

// NotLike creates a client-owned NotLike criterion.
func (c *Client) NotLike(left, right any) *criteria.Criterion {
	return criteria.NotLike(c.identity, left, right)
}
