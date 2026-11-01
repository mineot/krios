package expression

import "slices"

// And constructs an explicit conjunction, preserving child order and groups.
func And(predicates ...Predicate) Predicate {
	return group(andPredicate, predicates)
}

// Or constructs an explicit disjunction, preserving child order and groups.
func Or(predicates ...Predicate) Predicate {
	return group(orPredicate, predicates)
}

// Where constructs an explicit AND group. An empty group is invalid.
func Where(predicates ...Predicate) Predicate {
	return group(wherePredicate, predicates)
}

// On constructs an explicit AND group. An empty group is invalid.
func On(predicates ...Predicate) Predicate {
	return group(onPredicate, predicates)
}

func group(kind predicateKind, predicates []Predicate) Predicate {
	return Predicate{node: &predicateNode{kind: kind, children: slices.Clone(predicates)}}
}
