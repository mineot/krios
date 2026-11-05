package core

import "github.com/mineot/krios/internal/expressions"

// Ref creates a qualified identifier, interpreting dots as component separators.
func (c *Client) Ref(name string) *expressions.Reference {
	return expressions.NewReference(c.identity, name)
}

// RefParts creates an identifier from literal components, including literal '*'.
func (c *Client) RefParts(parts ...string) *expressions.Reference {
	return expressions.NewReferenceParts(c.identity, parts...)
}
