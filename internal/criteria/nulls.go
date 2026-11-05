package criteria

import "github.com/mineot/krios/internal/expressions"

func IsNull(owner *expressions.Identity, operand any) *Criterion {
	return predicate(owner, Null, operand)
}
func IsNotNull(owner *expressions.Identity, operand any) *Criterion {
	return predicate(owner, NotNull, operand)
}
