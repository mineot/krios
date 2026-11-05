package criteria

import "github.com/mineot/krios/internal/expressions"

func comparison(owner *expressions.Identity, operator Operator, left, right any) *Criterion {
	c := predicate(owner, operator, left)
	c.right = right
	c.record(validateValue(right, owner))
	return c
}
func Eq(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, Equal, left, right)
}
func Ne(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, NotEqual, left, right)
}
func Lt(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, Less, left, right)
}
func Lte(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, LessEqual, left, right)
}
func Gt(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, Greater, left, right)
}
func Gte(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, GreaterEqual, left, right)
}
func Like(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, LikeOperator, left, right)
}
func NotLike(owner *expressions.Identity, left, right any) *Criterion {
	return comparison(owner, NotLikeOperator, left, right)
}
