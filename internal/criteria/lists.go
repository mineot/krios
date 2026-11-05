package criteria

import (
	"github.com/mineot/krios/internal/expressions"
	"reflect"
)

func In(owner *expressions.Identity, operand, values any) *Criterion {
	return list(owner, InOperator, operand, values)
}
func NotIn(owner *expressions.Identity, operand, values any) *Criterion {
	return list(owner, NotInOperator, operand, values)
}
func list(owner *expressions.Identity, operator Operator, operand, values any) *Criterion {
	c := predicate(owner, operator, operand)
	// An untyped nil is a missing source, not a permanently invalid criterion:
	// a future SubSelect can supply that source without replacing the criterion.
	if values == nil {
		return c
	}
	v := reflect.ValueOf(values)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		c.record(invalid("IN requires a slice or array"))
		return c
	}
	c.hasList = true
	c.values = make([]any, v.Len())
	for i := range c.values {
		c.values[i] = v.Index(i).Interface()
	}
	return c
}
