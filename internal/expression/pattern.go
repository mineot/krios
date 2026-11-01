package expression

// Like constructs a pattern comparison with a parameterized pattern.
func Like(column, pattern string) Predicate {
	return comparison(likePredicate, Column(column), Value(pattern))
}

// NotLike constructs a negated pattern comparison.
func NotLike(column, pattern string) Predicate {
	return comparison(notLikePredicate, Column(column), Value(pattern))
}
