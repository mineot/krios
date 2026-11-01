package krios

import "github.com/mineot/krios/internal/expression"

// And constructs an explicit AND group; an empty group is invalid.
func And(predicates ...Predicate) Predicate {
	return expression.And(predicates...)
}

// Or constructs an explicit OR group; an empty group is invalid.
func Or(predicates ...Predicate) Predicate {
	return expression.Or(predicates...)
}

// Where constructs an explicit AND group; an empty group is invalid.
func Where(predicates ...Predicate) Predicate {
	return expression.Where(predicates...)
}

// On constructs an explicit AND group; an empty group is invalid.
func On(predicates ...Predicate) Predicate {
	return expression.On(predicates...)
}
