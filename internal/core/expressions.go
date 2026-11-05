package core

import "github.com/mineot/krios/internal/expressions"

// CountExpr creates a COUNT expression; "*" remains an unqualified wildcard.
func (c *Client) CountExpr(operand any) *expressions.Expression {
	return expressions.Count(c.identity, operand)
}

// MaxExpr creates a MAX expression.
func (c *Client) MaxExpr(operand any) *expressions.Expression {
	return expressions.Max(c.identity, operand)
}

// MinExpr creates a MIN expression.
func (c *Client) MinExpr(operand any) *expressions.Expression {
	return expressions.Min(c.identity, operand)
}

// AvgExpr creates an AVG expression.
func (c *Client) AvgExpr(operand any) *expressions.Expression {
	return expressions.Avg(c.identity, operand)
}

// SumExpr creates a SUM expression.
func (c *Client) SumExpr(operand any) *expressions.Expression {
	return expressions.Sum(c.identity, operand)
}
