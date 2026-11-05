package criteria

import "github.com/mineot/krios/internal/expressions"

func Between(owner *expressions.Identity, operand, lower, upper any) *Criterion {
	return interval(owner, BetweenOperator, operand, lower, upper)
}
func NotBetween(owner *expressions.Identity, operand, lower, upper any) *Criterion {
	return interval(owner, NotBetweenOperator, operand, lower, upper)
}
func interval(owner *expressions.Identity, operator Operator, operand, lower, upper any) *Criterion {
	c := predicate(owner, operator, operand)
	c.right = lower
	c.upper = upper
	c.record(validateValue(lower, owner))
	c.record(validateValue(upper, owner))
	return c
}
