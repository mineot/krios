package expression

// Truth describes proven constancy, not SQL's three-valued NULL semantics.
type Truth uint8

const (
	Unknown Truth = iota
	AlwaysTrue
	AlwaysFalse
)

// AnalyzePredicate returns independent constancy and validation results.
// A known truth value never suppresses an invalid child. Future write builders
// can apply their ErrMissingWhere priority without discarding validation errors.
func AnalyzePredicate(predicate Predicate) (Truth, error) {
	return classify(predicate), ValidatePredicate(predicate)
}

func classify(predicate Predicate) Truth {
	node := predicate.node

	if node == nil {
		return Unknown
	}

	switch node.kind {
	case inPredicate:
		if len(node.values) == 0 {
			return AlwaysFalse
		}
	case notInPredicate:
		if len(node.values) == 0 {
			return AlwaysTrue
		}
	case andPredicate, wherePredicate, onPredicate:
		return classifyGroup(node.children, AlwaysFalse, AlwaysTrue)
	case orPredicate:
		return classifyGroup(node.children, AlwaysTrue, AlwaysFalse)
	}

	return Unknown
}

func classifyGroup(children []Predicate, decisive, identity Truth) Truth {
	if len(children) == 0 {
		return Unknown
	}

	result := identity

	for _, child := range children {
		truth := classify(child)

		if truth == decisive {
			return decisive
		}

		if truth == Unknown {
			result = Unknown
		}
	}

	return result
}
